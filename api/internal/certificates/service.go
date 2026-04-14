package certificates

import (
	"context"
	"errors"
	"log"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
)

var (
	ErrInvalidInput                   = errors.New("invalid input")
	ErrInvalidRegistryDate            = errors.New("invalid registry chronology")
	ErrCertificateTranslationNotFound = errors.New("certificate translation not found")
	ErrRegistryNumberTaken            = errors.New("registry number already taken")
)

type CreateCertificateInput struct {
	StudentID       int64
	CourseID        int64
	CertificateDate string
	CourseDateStart string
	CourseDateEnd   *string
	RegistryYear    int64
	RegistryNumber  int32
	LanguageCode    string
}

type UpdateCertificateInput struct {
	StudentID       int64
	CertificateDate string
	CourseDateStart string
	CourseDateEnd   *string
}

type CreateCertificateResult struct {
	ID int64
}

type txScope struct {
	queries  *dbsqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	pool     *pgxpool.Pool
	queries  *dbsqlc.Queries
	recorder *auditlog.Recorder
	beginTx  func(context.Context) (txScope, error)
}
type studentSnapshot struct {
	FirstName   string
	SecondName  *string
	LastName    string
	BirthDate   time.Time
	BirthPlace  string
	Pesel       *string
	CompanyName *string
}
type courseSnapshot struct {
	Name       string
	Symbol     string
	ExpiryTime *string
	Program    []byte
	FrontPage  string
}

func NewService(pool *pgxpool.Pool, queries *dbsqlc.Queries, recorder *auditlog.Recorder) *Service {
	return &Service{
		pool:     pool,
		queries:  queries,
		recorder: recorder,
		beginTx: func(ctx context.Context) (txScope, error) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return txScope{}, err
			}
			return newTxScope(tx, queries), nil
		},
	}
}

func (s *Service) Create(ctx context.Context, input CreateCertificateInput) (CreateCertificateResult, error) {
	if err := validateCreateInput(input); err != nil {
		return CreateCertificateResult{}, err
	}

	certificateDate, err := parseDate(input.CertificateDate)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	courseDateStart, err := parseDate(input.CourseDateStart)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	courseDateEnd, err := parseOptionalDate(input.CourseDateEnd)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	if courseDateEnd.Valid && courseDateEnd.Time.Before(courseDateStart.Time) {
		return CreateCertificateResult{}, ErrInvalidInput
	}

	if input.StudentID > math.MaxInt32 {
		return CreateCertificateResult{}, ErrInvalidInput
	}
	languageCode := normalizeLanguageCode(input.LanguageCode)
	rows, err := s.queries.ListRegistryDatesForCourseYear(ctx, dbsqlc.ListRegistryDatesForCourseYearParams{
		CourseID: input.CourseID,
		Year:     input.RegistryYear,
	})
	if err != nil {
		return CreateCertificateResult{}, err
	}

	if err := validateRegistryChronology(rows, input.RegistryNumber, certificateDate); err != nil {
		return CreateCertificateResult{}, err
	}
	student, err := s.queries.GetStudentByID(ctx, input.StudentID)
	if err != nil {
		return CreateCertificateResult{}, err
	}
	course, err := s.queries.GetCourseByID(ctx, input.CourseID)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	var translation *dbsqlc.GetCourseCertificateTranslationByCourseAndLanguageRow
	if languageCode != "pl" {
		row, err := s.queries.GetCourseCertificateTranslationByCourseAndLanguage(ctx, dbsqlc.GetCourseCertificateTranslationByCourseAndLanguageParams{
			CourseID:     input.CourseID,
			LanguageCode: languageCode,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return CreateCertificateResult{}, ErrCertificateTranslationNotFound
			}
			return CreateCertificateResult{}, err
		}
		translation = &row
	}
	studentSnapshot, err := buildStudentSnapshot(student)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	courseSnapshot := buildCourseSnapshot(course, translation, languageCode)

	exists, err := s.queries.ActiveRegistryNumberExistsForCourseYear(ctx, dbsqlc.ActiveRegistryNumberExistsForCourseYearParams{
		CourseID: input.CourseID,
		Year:     input.RegistryYear,
		Number:   input.RegistryNumber,
	})
	if err != nil {
		return CreateCertificateResult{}, err
	}
	if exists {
		return CreateCertificateResult{}, ErrRegistryNumberTaken
	}
	tx, err := s.beginTx(ctx)
	if err != nil {
		return CreateCertificateResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			err = tx.rollback(ctx)
			if err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	registryID, err := tx.queries.CreateRegistry(ctx, dbsqlc.CreateRegistryParams{
		CourseID: input.CourseID,
		Year:     input.RegistryYear,
		Number:   input.RegistryNumber,
	})
	if err != nil {
		return CreateCertificateResult{}, err
	}

	certificateParams := toCreateCertificateParams(
		input,
		registryID,
		certificateDate,
		courseDateStart,
		courseDateEnd,
		studentSnapshot,
		courseSnapshot,
		languageCode)
	certificateID, err := tx.queries.CreateCertificate(ctx, certificateParams)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	if s.recorder != nil {
		createdCertificate, err := tx.queries.GetCertificateByID(ctx, certificateID)
		if err != nil {
			return CreateCertificateResult{}, err
		}

		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "certificate",
			EntityID:   certificateID,
			Action:     "create",
			Before:     nil,
			After:      mapCertificateDetailsResponse(createdCertificate, nil),
			Metadata:   nil,
		}); err != nil {
			return CreateCertificateResult{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return CreateCertificateResult{}, err
	}
	committed = true

	return CreateCertificateResult{ID: certificateID}, nil
}

func (s *Service) Update(ctx context.Context, certificateID int64, input UpdateCertificateInput) (dbsqlc.UpdateCertificateRow, error) {
	if certificateID <= 0 {
		return dbsqlc.UpdateCertificateRow{}, ErrInvalidInput
	}

	if err := validateUpdateInput(input); err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}

	certificateDate, err := parseDate(input.CertificateDate)
	if err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}

	courseDateStart, err := parseDate(input.CourseDateStart)
	if err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}

	courseDateEnd, err := parseOptionalDate(input.CourseDateEnd)
	if err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}

	if courseDateEnd.Valid && courseDateEnd.Time.Before(courseDateStart.Time) {
		return dbsqlc.UpdateCertificateRow{}, ErrInvalidInput
	}

	student, err := s.queries.GetStudentByID(ctx, input.StudentID)
	if err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}

	studentSnapshot, err := buildStudentSnapshot(student)
	if err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}
	committed := false
	defer func() {
		if !committed {
			err = tx.rollback(ctx)
			if err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	beforeCertificate, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}

	updatedCertificate, err := tx.queries.UpdateCertificate(ctx, toUpdateCertificateParams(
		certificateID,
		input.StudentID,
		certificateDate,
		courseDateStart,
		courseDateEnd,
		studentSnapshot,
	))
	if err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}

	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "certificate",
			EntityID:   certificateID,
			Action:     "update",
			Before:     mapCertificateDetailsResponse(beforeCertificate, nil),
			After:      mapCertificateDetailsResponse(dbsqlc.GetCertificateByIDRow(updatedCertificate), nil),
			Metadata:   nil,
		}); err != nil {
			return dbsqlc.UpdateCertificateRow{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return dbsqlc.UpdateCertificateRow{}, err
	}
	committed = true

	return updatedCertificate, nil
}

func newTxScope(tx pgx.Tx, queries *dbsqlc.Queries) txScope {
	return txScope{
		queries:  queries.WithTx(tx),
		commit:   tx.Commit,
		rollback: tx.Rollback,
	}
}

func parseDate(value string) (pgtype.Date, error) {
	date, err := time.Parse(time.DateOnly, strings.TrimSpace(value))
	if err != nil {
		return pgtype.Date{}, ErrInvalidInput
	}
	return pgtype.Date{
		Time:             date,
		InfinityModifier: 0,
		Valid:            true,
	}, nil
}

func parseOptionalDate(value *string) (pgtype.Date, error) {
	if value == nil {
		return pgtype.Date{
			Time:             time.Time{},
			InfinityModifier: 0,
			Valid:            false,
		}, nil
	}
	return parseDate(*value)
}

func validateCreateInput(input CreateCertificateInput) error {
	if input.StudentID <= 0 ||
		input.CourseID <= 0 ||
		input.RegistryYear <= 0 ||
		input.RegistryNumber <= 0 ||
		strings.TrimSpace(input.CertificateDate) == "" ||
		strings.TrimSpace(input.CourseDateStart) == "" {
		return ErrInvalidInput
	}
	return nil
}

func validateUpdateInput(input UpdateCertificateInput) error {
	if input.StudentID <= 0 ||
		input.StudentID > math.MaxInt32 ||
		strings.TrimSpace(input.CertificateDate) == "" ||
		strings.TrimSpace(input.CourseDateStart) == "" {
		return ErrInvalidInput
	}

	return nil
}

func validateRegistryChronology(
	rows []dbsqlc.ListRegistryDatesForCourseYearRow,
	registryNumber int32,
	certificateDate pgtype.Date,
) error {
	if !certificateDate.Valid {
		return ErrInvalidInput
	}

	if len(rows) == 0 {
		return nil
	}

	minIdx := -1
	maxIdx := len(rows)

	for i, row := range rows {
		if row.RegistryNumber <= registryNumber && minIdx <= i {
			minIdx = i
		}
		if row.RegistryNumber >= registryNumber && maxIdx >= i {
			maxIdx = i
		}
	}

	minDate := certificateDate.Time
	maxDate := certificateDate.Time

	if minIdx >= 0 {
		minDate = rows[minIdx].CertificateDate.Time
	}
	if maxIdx < len(rows) {
		maxDate = rows[maxIdx].CertificateDate.Time
	}

	if certificateDate.Time.Before(minDate) || certificateDate.Time.After(maxDate) {
		return ErrInvalidRegistryDate
	}

	return nil
}

func normalizeLanguageCode(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return "pl"
	}
	return normalized
}

func buildStudentSnapshot(student dbsqlc.GetStudentByIDRow) (studentSnapshot, error) {
	if !student.Birthdate.Valid {
		return studentSnapshot{}, ErrInvalidInput
	}
	return studentSnapshot{
		FirstName:   strings.TrimSpace(student.Firstname),
		SecondName:  pgutil.NullableString(student.Secondname),
		LastName:    strings.TrimSpace(student.Lastname),
		BirthDate:   student.Birthdate.Time,
		BirthPlace:  strings.TrimSpace(student.Birthplace),
		Pesel:       pgutil.NullableString(student.Pesel),
		CompanyName: pgutil.NullableString(student.CompanyName),
	}, nil
}

func buildCourseSnapshot(
	course dbsqlc.Course,
	translation *dbsqlc.GetCourseCertificateTranslationByCourseAndLanguageRow,
	languageCode string,
) courseSnapshot {
	snapshot := courseSnapshot{
		Name:       strings.TrimSpace(course.Name),
		Symbol:     strings.TrimSpace(course.Symbol),
		ExpiryTime: pgutil.NullableString(course.Expirytime),
		Program:    append([]byte(nil), course.Courseprogram...),
		FrontPage:  strings.TrimSpace(course.Certfrontpage.String),
	}
	if languageCode == "pl" || translation == nil {
		return snapshot
	}
	snapshot.Name = strings.TrimSpace(translation.CourseName)
	snapshot.Program = []byte(translation.CourseProgram)
	snapshot.FrontPage = strings.TrimSpace(translation.CertFrontPage)
	return snapshot
}

func toCreateCertificateParams(
	input CreateCertificateInput,
	registryID int64,
	certificateDate pgtype.Date,
	courseDateStart pgtype.Date,
	courseDateEnd pgtype.Date,
	student studentSnapshot,
	course courseSnapshot,
	languageCode string,
) dbsqlc.CreateCertificateParams {
	return dbsqlc.CreateCertificateParams{
		Date:                      certificateDate,
		StudentID:                 validation.Int64ToInt32(input.StudentID),
		CourseDateStart:           courseDateStart,
		CourseDateEnd:             courseDateEnd,
		RegistryID:                registryID,
		LanguageCode:              languageCode,
		StudentFirstnameSnapshot:  student.FirstName,
		StudentSecondnameSnapshot: pgutil.OptionalText(student.SecondName),
		StudentLastnameSnapshot:   student.LastName,
		StudentBirthdateSnapshot: pgtype.Date{
			Time:  student.BirthDate,
			Valid: true,
		},
		StudentBirthplaceSnapshot: student.BirthPlace,
		StudentPeselSnapshot:      pgutil.OptionalText(student.Pesel),
		CompanyNameSnapshot:       pgutil.OptionalText(student.CompanyName),
		CourseNameSnapshot:        course.Name,
		CourseSymbolSnapshot:      course.Symbol,
		CourseExpiryTimeSnapshot:  pgutil.OptionalText(course.ExpiryTime),
		CourseProgramSnapshot:     course.Program,
		CertFrontPageSnapshot:     course.FrontPage,
	}
}

func toUpdateCertificateParams(
	certificateID int64,
	studentID int64,
	certificateDate pgtype.Date,
	courseDateStart pgtype.Date,
	courseDateEnd pgtype.Date,
	student studentSnapshot,
) dbsqlc.UpdateCertificateParams {
	return dbsqlc.UpdateCertificateParams{
		Date:                      certificateDate,
		StudentID:                 validation.Int64ToInt32(studentID),
		CourseDateStart:           courseDateStart,
		CourseDateEnd:             courseDateEnd,
		StudentFirstnameSnapshot:  student.FirstName,
		StudentSecondnameSnapshot: pgutil.OptionalText(student.SecondName),
		StudentLastnameSnapshot:   student.LastName,
		StudentBirthdateSnapshot: pgtype.Date{
			Time:  student.BirthDate,
			Valid: true,
		},
		StudentBirthplaceSnapshot: student.BirthPlace,
		StudentPeselSnapshot:      pgutil.OptionalText(student.Pesel),
		CompanyNameSnapshot:       pgutil.OptionalText(student.CompanyName),
		CertificateID:             certificateID,
	}
}
