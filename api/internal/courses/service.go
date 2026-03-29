package courses

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

type CourseTranslationInput struct {
	LanguageCode  string
	CourseName    string
	CourseProgram string
	CertFrontPage string
}

var supportedTranslationLanguageCodes = map[string]struct{}{
	"en": {},
	"de": {},
	"uk": {},
	"cs": {},
	"sk": {},
	"lt": {},
}

type CreateCourseInput struct {
	MainName                string
	Name                    string
	Symbol                  string
	ExpiryTime              string
	CourseProgram           string
	CertFrontPage           string
	CertificateTranslations []CourseTranslationInput
}
type UpdateCourseInput = CreateCourseInput

type Service struct {
	pool     *pgxpool.Pool
	queries  *sqlc.Queries
	recorder *auditlog.Recorder
	beginTx  func(context.Context) (txScope, error)
}
type txScope struct {
	queries  *sqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

func newTxScope(tx pgx.Tx, queries *sqlc.Queries) txScope {
	return txScope{
		queries:  queries.WithTx(tx),
		commit:   tx.Commit,
		rollback: tx.Rollback,
	}
}

var (
	ErrInvalidInput             = errors.New("invalid input")
	ErrDatabaseTransactionError = errors.New("database error")
)

func NewService(pool *pgxpool.Pool, queries *sqlc.Queries, recorder *auditlog.Recorder) *Service {
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

func (s *Service) Create(ctx context.Context, input CreateCourseInput) (CourseDetailDTO, error) {
	if err := validateCourseInput(input); err != nil {
		return CourseDetailDTO{}, err
	}
	translations, err := normalizeTranslationInput(input.CertificateTranslations)
	if err != nil {
		return CourseDetailDTO{}, err
	}
	expiryTime, err := normalizeExpiryTime(input.ExpiryTime)
	if err != nil {
		return CourseDetailDTO{}, err
	}
	tx, err := s.beginTx(ctx)
	if err != nil {
		return CourseDetailDTO{}, err
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
	courseParams := sqlc.CreateCourseParams{
		Mainname:      pgtype.Text{String: input.MainName, Valid: true},
		Name:          input.Name,
		Symbol:        input.Symbol,
		Expirytime:    pgtype.Text{String: expiryTime, Valid: true},
		Courseprogram: []byte(input.CourseProgram),
		Certfrontpage: pgtype.Text{String: input.CertFrontPage, Valid: true},
	}
	row, err := tx.queries.CreateCourse(ctx, courseParams)
	if err != nil {
		return CourseDetailDTO{}, err
	}

	if err := syncCourseCertificateTranslations(ctx, tx.queries, row.ID, translations); err != nil {
		return CourseDetailDTO{}, ErrDatabaseTransactionError
	}
	courseTranslations, err := tx.queries.ListCourseCertificateTranslationsByCourseID(ctx, row.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CourseDetailDTO{}, err
		}
		return CourseDetailDTO{}, ErrDatabaseTransactionError
	}
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "course",
			EntityID:   row.ID,
			Action:     "create",
			Before:     nil,
			After:      makeCourseDetailDTO(row, courseTranslations),
			Metadata:   nil,
		}); err != nil {
			return CourseDetailDTO{}, err
		}
	}
	if err := tx.commit(ctx); err != nil {
		return CourseDetailDTO{}, err
	}
	committed = true
	return makeCourseDetailDTO(row, courseTranslations), nil
}

func (s *Service) Update(ctx context.Context, courseID int64, input UpdateCourseInput) (CourseDetailDTO, error) {
	if courseID <= 0 {
		return CourseDetailDTO{}, ErrInvalidInput
	}

	if err := validateCourseInput(input); err != nil {
		return CourseDetailDTO{}, err
	}
	translations, err := normalizeTranslationInput(input.CertificateTranslations)
	if err != nil {
		return CourseDetailDTO{}, err
	}
	expiryTime, err := normalizeExpiryTime(input.ExpiryTime)
	if err != nil {
		return CourseDetailDTO{}, err
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return CourseDetailDTO{}, err
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
	beforeCourse, err := tx.queries.GetCourseByID(ctx, courseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CourseDetailDTO{}, err
		}
		return CourseDetailDTO{}, ErrDatabaseTransactionError
	}
	beforeTranslations, err := tx.queries.ListCourseCertificateTranslationsByCourseID(ctx, courseID)
	if err != nil {
		return CourseDetailDTO{}, ErrDatabaseTransactionError
	}
	before := makeCourseDetailDTO(beforeCourse, beforeTranslations)
	row, err := tx.queries.UpdateCourse(ctx, sqlc.UpdateCourseParams{
		ID:            courseID,
		Mainname:      pgtype.Text{String: input.MainName, Valid: true},
		Name:          input.Name,
		Symbol:        input.Symbol,
		Expirytime:    pgtype.Text{String: expiryTime, Valid: true},
		Courseprogram: []byte(input.CourseProgram),
		Certfrontpage: pgtype.Text{String: input.CertFrontPage, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CourseDetailDTO{}, err
		}
		return CourseDetailDTO{}, ErrDatabaseTransactionError
	}
	if err := syncCourseCertificateTranslations(ctx, tx.queries, courseID, translations); err != nil {
		return CourseDetailDTO{}, ErrDatabaseTransactionError
	}

	courseTranslations, err := tx.queries.ListCourseCertificateTranslationsByCourseID(ctx, courseID)
	if err != nil {
		return CourseDetailDTO{}, ErrDatabaseTransactionError
	}

	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "course",
			EntityID:   courseID,
			Action:     "update",
			Before:     before,
			After:      makeCourseDetailDTO(row, courseTranslations),
			Metadata:   nil,
		}); err != nil {
			return CourseDetailDTO{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return CourseDetailDTO{}, err
	}
	committed = true

	return makeCourseDetailDTO(row, courseTranslations), nil
}

func validateCourseInput(input CreateCourseInput) error {
	mainName := strings.TrimSpace(input.MainName)
	name := strings.TrimSpace(input.Name)
	symbol := strings.TrimSpace(input.Symbol)
	courseProgram := strings.TrimSpace(input.CourseProgram)
	certFrontPage := strings.TrimSpace(input.CertFrontPage)
	if mainName == "" || name == "" || symbol == "" || courseProgram == "" || certFrontPage == "" {
		return ErrInvalidInput
	}
	return nil
}

func normalizeTranslationInput(input []CourseTranslationInput) ([]CourseTranslationInput, error) {
	output := make([]CourseTranslationInput, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, translation := range input {
		courseName := strings.TrimSpace(translation.CourseName)
		courseProgram := strings.TrimSpace(translation.CourseProgram)
		certFrontPage := strings.TrimSpace(translation.CertFrontPage)
		languageCode := strings.ToLower(strings.TrimSpace(translation.LanguageCode))
		if languageCode == "" || courseProgram == "" || certFrontPage == "" || courseName == "" {
			return nil, ErrInvalidInput
		}
		if languageCode == "pl" {
			return nil, ErrInvalidInput
		}
		if _, supported := supportedTranslationLanguageCodes[languageCode]; !supported {
			return nil, ErrInvalidInput
		}
		if _, exists := seen[languageCode]; exists {
			return nil, ErrInvalidInput
		}
		seen[languageCode] = struct{}{}
		output = append(output, CourseTranslationInput{
			LanguageCode:  languageCode,
			CourseName:    courseName,
			CourseProgram: courseProgram,
			CertFrontPage: certFrontPage,
		})

	}
	return output, nil
}

func syncCourseCertificateTranslations(
	ctx context.Context,
	q *sqlc.Queries,
	courseID int64,
	translations []CourseTranslationInput,
) error {
	wantedTranslations := make(map[string]CourseTranslationInput, len(translations))
	for _, translation := range translations {
		wantedTranslations[translation.LanguageCode] = translation
		_, err := q.UpsertCourseCertificateTranslation(ctx, sqlc.UpsertCourseCertificateTranslationParams{
			CourseID:      courseID,
			LanguageCode:  translation.LanguageCode,
			CourseName:    translation.CourseName,
			CourseProgram: []byte(translation.CourseProgram),
			CertFrontPage: translation.CertFrontPage,
		})
		if err != nil {
			return err
		}
	}

	existingTranslations, err := q.ListCourseCertificateTranslationsByCourseID(ctx, courseID)
	if err != nil {
		return err
	}

	for _, translation := range existingTranslations {
		if _, exists := wantedTranslations[translation.LanguageCode]; exists {
			continue
		}
		_, err := q.DeleteCourseCertificateTranslation(ctx, sqlc.DeleteCourseCertificateTranslationParams{
			CourseID:     courseID,
			LanguageCode: translation.LanguageCode,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func normalizeExpiryTime(exp string) (string, error) {
	expiryValue := strings.TrimSpace(exp)

	expiryInt, err := strconv.Atoi(expiryValue)
	if err != nil || expiryInt < 0 {
		return "", ErrInvalidInput
	}
	return expiryValue, nil
}
