package students

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

var ErrInvalidInput = errors.New("invalid input")

// ErrCompanyNotFound oznacza, że companyId z ciała żądania nie wskazuje istniejącej
// firmy. Wcześniej naruszenie klucza obcego docierało do klienta jako 500.
var ErrCompanyNotFound = errors.New("company not found")

// studentCompanyForeignKeys to nazwy klucza obcego students.company_id. Bazy
// założone ze starego schematu mają oba ograniczenia (fk_company oraz dodane
// migracją 0013 students_company_id_fkey), a PostgreSQL zgłasza to, które sprawdzi
// jako pierwsze - dlatego dopasowanie do jednej nazwy nie działało na takiej bazie.
var studentCompanyForeignKeys = map[string]struct{}{
	"students_company_id_fkey": {},
	"fk_company":               {},
}

func companyNotFoundAs(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		if _, ok := studentCompanyForeignKeys[pgErr.ConstraintName]; ok {
			return ErrCompanyNotFound
		}
	}
	return err
}

// ErrDuplicateStudent oznacza, że istnieje już kursant o tym samym imieniu, nazwisku
// i dacie urodzenia (bez rozróżniania wielkości liter i spacji na brzegach).
var ErrDuplicateStudent = errors.New("student with the same name and birth date already exists")

// studentPersonUniqueConstraints to ograniczenia unikalności osoby: stara nazwa,
// nazwa nadana w 0017 i indeks z 0018. Służą jako zabezpieczenie przed wyścigiem
// dwóch równoczesnych zapisów, gdy sprawdzenie w ensureNoDuplicateStudent nie wystarczy.
var studentPersonUniqueConstraints = map[string]struct{}{
	"unique_user_lastname_birthdate":  {},
	"students_person_unique":          {},
	"students_person_normalized_uidx": {},
}

// studentWriteError tłumaczy błędy bazy przy zapisie kursanta na błędy domenowe.
func studentWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if _, ok := studentPersonUniqueConstraints[pgErr.ConstraintName]; ok {
			return ErrDuplicateStudent
		}
	}
	return companyNotFoundAs(err)
}

// ensureNoDuplicateStudent blokuje duplikat osoby, także różniący się tylko wielkością
// liter lub spacjami. Ograniczenie w bazie (0017) łapie wyłącznie identyczny zapis;
// pełną regułę w bazie wprowadza 0018, możliwa dopiero po uporządkowaniu istniejących
// duplikatów. excludeID pomija edytowanego kursanta.
func ensureNoDuplicateStudent(ctx context.Context, q *dbsqlc.Queries, firstname, lastname string, birthdate pgtype.Date, excludeID pgtype.Int8) error {
	_, err := q.FindDuplicateStudent(ctx, dbsqlc.FindDuplicateStudentParams{
		Firstname: firstname,
		Lastname:  lastname,
		Birthdate: birthdate,
		ExcludeID: excludeID,
	})
	if err == nil {
		return ErrDuplicateStudent
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

// personKeyChanged mówi, czy edycja zmienia dane identyfikujące osobę. Porównanie
// odpowiada normalizacji z FindDuplicateStudent.
func personKeyChanged(before dbsqlc.GetStudentByIDRow, params dbsqlc.UpdateStudentParams) bool {
	normalize := func(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
	return normalize(before.Firstname) != normalize(params.Firstname) ||
		normalize(before.Lastname) != normalize(params.Lastname) ||
		!before.Birthdate.Time.Equal(params.Birthdate.Time)
}

// MaxExternalIDLength to limit długości identyfikatora platformy (CHECK w migracji 0020).
const MaxExternalIDLength = 64

// NaturalKeyConflictError - dane z PUT by-external-id wskazują na osobę, która już
// istnieje jako inny rekord. ID to kolidujący rekord; 0, gdy nie udało się go ustalić.
type NaturalKeyConflictError struct {
	ID int64
}

func (e *NaturalKeyConflictError) Error() string {
	return "student with the same natural key already exists"
}

func (e *NaturalKeyConflictError) Unwrap() error {
	return ErrDuplicateStudent
}

// findDuplicateStudentID zwraca id innego kursanta o tym samym kluczu naturalnym albo 0.
func findDuplicateStudentID(ctx context.Context, q *dbsqlc.Queries, firstname, lastname string, birthdate pgtype.Date, excludeID pgtype.Int8) (int64, error) {
	id, err := q.FindDuplicateStudent(ctx, dbsqlc.FindDuplicateStudentParams{
		Firstname: firstname,
		Lastname:  lastname,
		Birthdate: birthdate,
		ExcludeID: excludeID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

type txScope struct {
	queries  *dbsqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	queries   *dbsqlc.Queries
	recorder  *auditlog.Recorder
	beginTxFn func(context.Context) (txScope, error)
}

func NewService(pool *pgxpool.Pool, queries *dbsqlc.Queries, recorder *auditlog.Recorder) *Service {
	return &Service{
		queries:  queries,
		recorder: recorder,
		beginTxFn: func(ctx context.Context) (txScope, error) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return txScope{}, err
			}

			return txScope{
				queries:  queries.WithTx(tx),
				commit:   tx.Commit,
				rollback: tx.Rollback,
			}, nil
		},
	}
}

func (s *Service) Create(ctx context.Context, req CreateStudentRequest) (StudentDetailsDTO, error) {
	params, err := buildCreateStudentParams(req)
	if err != nil {
		return StudentDetailsDTO{}, err
	}

	tx, err := s.beginTxFn(ctx)
	if err != nil {
		return StudentDetailsDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	if err := ensureNoDuplicateStudent(ctx, tx.queries, params.Firstname, params.Lastname, params.Birthdate, pgtype.Int8{}); err != nil {
		return StudentDetailsDTO{}, err
	}

	createdStudent, err := tx.queries.CreateStudent(ctx, params)
	if err != nil {
		return StudentDetailsDTO{}, studentWriteError(err)
	}

	createdSnapshot := mapCreateStudentRow(createdStudent)
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "student",
			EntityID:   createdStudent.ID,
			Action:     "create",
			Before:     nil,
			After:      createdSnapshot,
			Metadata:   nil,
		}); err != nil {
			return StudentDetailsDTO{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return StudentDetailsDTO{}, err
	}
	committed = true

	return createdSnapshot, nil
}

func (s *Service) Update(ctx context.Context, studentID int64, req UpdateStudentRequest) (StudentDetailsDTO, error) {
	if studentID <= 0 {
		return StudentDetailsDTO{}, ErrInvalidInput
	}

	params, err := buildUpdateStudentParams(studentID, req)
	if err != nil {
		return StudentDetailsDTO{}, err
	}

	tx, err := s.beginTxFn(ctx)
	if err != nil {
		return StudentDetailsDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	beforeStudent, err := tx.queries.GetStudentByID(ctx, studentID)
	if err != nil {
		return StudentDetailsDTO{}, err
	}

	// Sprawdzamy duplikat tylko przy zmianie danych osoby - inaczej rekordów z istniejących
	// grup duplikatów nie dałoby się edytować (np. poprawić telefonu) przed ich scaleniem.
	if personKeyChanged(beforeStudent, params) {
		if err := ensureNoDuplicateStudent(ctx, tx.queries, params.Firstname, params.Lastname, params.Birthdate, pgtype.Int8{Int64: studentID, Valid: true}); err != nil {
			return StudentDetailsDTO{}, err
		}
	}

	updatedStudent, err := tx.queries.UpdateStudent(ctx, params)
	if err != nil {
		return StudentDetailsDTO{}, studentWriteError(err)
	}

	beforeSnapshot := mapStudentGetRow(beforeStudent)
	afterSnapshot := mapStudentDetailsRow(updatedStudent)
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "student",
			EntityID:   studentID,
			Action:     "update",
			Before:     beforeSnapshot,
			After:      afterSnapshot,
			Metadata:   nil,
		}); err != nil {
			return StudentDetailsDTO{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return StudentDetailsDTO{}, err
	}
	committed = true

	return afterSnapshot, nil
}

// UpsertByExternalID zakłada kursanta o danym identyfikatorze platformy albo nadpisuje
// istniejącego (jak PATCH). created mówi, czy rekord powstał. Kolizja klucza naturalnego
// z innym rekordem kończy się *NaturalKeyConflictError - bez tworzenia duplikatu
// i bez zmiany cudzego rekordu.
func (s *Service) UpsertByExternalID(ctx context.Context, externalID string, req CreateStudentRequest) (StudentDetailsDTO, bool, error) {
	if externalID == "" || len([]rune(externalID)) > MaxExternalIDLength {
		return StudentDetailsDTO{}, false, ErrInvalidInput
	}
	params, err := buildCreateStudentParams(req)
	if err != nil {
		return StudentDetailsDTO{}, false, err
	}
	externalIDText := pgtype.Text{String: externalID, Valid: true}

	student, created, err := s.upsertByExternalID(ctx, externalIDText, params)
	if err != nil && errors.Is(err, ErrDuplicateStudent) {
		var conflict *NaturalKeyConflictError
		if !errors.As(err, &conflict) {
			// Wyścig z równoległym zapisem tej samej osoby: sprawdzenie przeszło, ale
			// ograniczenie w bazie odrzuciło zapis. Transakcja jest już wycofana, więc
			// kolidujący rekord szukamy poza nią.
			conflict = &NaturalKeyConflictError{}
			if s.queries != nil {
				excludeID := pgtype.Int8{}
				if existingID, lookupErr := s.queries.GetStudentIDByExternalID(ctx, externalIDText); lookupErr == nil {
					excludeID = pgtype.Int8{Int64: existingID, Valid: true}
				}
				if id, lookupErr := findDuplicateStudentID(ctx, s.queries, params.Firstname, params.Lastname, params.Birthdate, excludeID); lookupErr == nil {
					conflict.ID = id
				}
			}
			err = conflict
		}
	}
	return student, created, err
}

func (s *Service) upsertByExternalID(ctx context.Context, externalID pgtype.Text, params dbsqlc.CreateStudentParams) (StudentDetailsDTO, bool, error) {
	tx, err := s.beginTxFn(ctx)
	if err != nil {
		return StudentDetailsDTO{}, false, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	if err := tx.queries.AcquireStudentExternalIDLock(ctx, externalID.String); err != nil {
		return StudentDetailsDTO{}, false, err
	}

	existingID, err := tx.queries.GetStudentIDByExternalID(ctx, externalID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		existingID = 0
	case err != nil:
		return StudentDetailsDTO{}, false, err
	}

	var (
		result StudentDetailsDTO
		entry  auditlog.Entry
	)
	if existingID == 0 {
		duplicateID, err := findDuplicateStudentID(ctx, tx.queries, params.Firstname, params.Lastname, params.Birthdate, pgtype.Int8{})
		if err != nil {
			return StudentDetailsDTO{}, false, err
		}
		if duplicateID != 0 {
			return StudentDetailsDTO{}, false, &NaturalKeyConflictError{ID: duplicateID}
		}

		createdStudent, err := tx.queries.CreateStudent(ctx, params)
		if err != nil {
			return StudentDetailsDTO{}, false, studentWriteError(err)
		}
		if err := tx.queries.SetStudentExternalID(ctx, dbsqlc.SetStudentExternalIDParams{
			ExternalID: externalID,
			ID:         createdStudent.ID,
		}); err != nil {
			return StudentDetailsDTO{}, false, err
		}
		createdStudent.ExternalID = externalID
		result = mapCreateStudentRow(createdStudent)
		entry = auditlog.Entry{EntityType: "student", EntityID: createdStudent.ID, Action: "create", After: result}
	} else {
		updateParams := dbsqlc.UpdateStudentParams{
			Firstname:     params.Firstname,
			Lastname:      params.Lastname,
			Secondname:    params.Secondname,
			Birthdate:     params.Birthdate,
			Birthplace:    params.Birthplace,
			Pesel:         params.Pesel,
			Addressstreet: params.Addressstreet,
			Addresscity:   params.Addresscity,
			Addresszip:    params.Addresszip,
			Telephoneno:   params.Telephoneno,
			CompanyID:     params.CompanyID,
			StudentID:     existingID,
		}
		beforeStudent, err := tx.queries.GetStudentByID(ctx, existingID)
		if err != nil {
			return StudentDetailsDTO{}, false, err
		}
		if personKeyChanged(beforeStudent, updateParams) {
			duplicateID, err := findDuplicateStudentID(ctx, tx.queries, params.Firstname, params.Lastname, params.Birthdate, pgtype.Int8{Int64: existingID, Valid: true})
			if err != nil {
				return StudentDetailsDTO{}, false, err
			}
			if duplicateID != 0 {
				return StudentDetailsDTO{}, false, &NaturalKeyConflictError{ID: duplicateID}
			}
		}
		updatedStudent, err := tx.queries.UpdateStudent(ctx, updateParams)
		if err != nil {
			return StudentDetailsDTO{}, false, studentWriteError(err)
		}
		result = mapStudentDetailsRow(updatedStudent)
		entry = auditlog.Entry{EntityType: "student", EntityID: existingID, Action: "update", Before: mapStudentGetRow(beforeStudent), After: result}
	}

	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, entry); err != nil {
			return StudentDetailsDTO{}, false, err
		}
	}
	if err := tx.commit(ctx); err != nil {
		return StudentDetailsDTO{}, false, err
	}
	committed = true
	return result, existingID == 0, nil
}

func buildCreateStudentParams(req CreateStudentRequest) (dbsqlc.CreateStudentParams, error) {
	payload, birthDate, err := normalizeStudentPayload(req.studentPayload)
	if err != nil {
		return dbsqlc.CreateStudentParams{}, err
	}

	return dbsqlc.CreateStudentParams{
		Firstname:     payload.FirstName,
		Lastname:      payload.LastName,
		Secondname:    pgutil.OptionalText(payload.SecondName),
		Birthdate:     pgtype.Date{Time: birthDate, Valid: true},
		Birthplace:    payload.BirthPlace,
		Pesel:         pgutil.OptionalText(payload.Pesel),
		Addressstreet: pgutil.OptionalText(payload.AddressStreet),
		Addresscity:   pgutil.OptionalText(payload.AddressCity),
		Addresszip:    pgutil.OptionalText(payload.AddressZip),
		Telephoneno:   pgutil.OptionalText(payload.Telephone),
		CompanyID:     pgutil.OptionalInt8(payload.CompanyID),
	}, nil
}

func buildUpdateStudentParams(studentID int64, req UpdateStudentRequest) (dbsqlc.UpdateStudentParams, error) {
	params, err := buildCreateStudentParams(CreateStudentRequest(req))
	if err != nil {
		return dbsqlc.UpdateStudentParams{}, err
	}

	return dbsqlc.UpdateStudentParams{
		Firstname:     params.Firstname,
		Lastname:      params.Lastname,
		Secondname:    params.Secondname,
		Birthdate:     params.Birthdate,
		Birthplace:    params.Birthplace,
		Pesel:         params.Pesel,
		Addressstreet: params.Addressstreet,
		Addresscity:   params.Addresscity,
		Addresszip:    params.Addresszip,
		Telephoneno:   params.Telephoneno,
		CompanyID:     params.CompanyID,
		StudentID:     studentID,
	}, nil
}

func normalizeStudentPayload(payload studentPayload) (studentPayload, time.Time, error) {
	firstName := strings.TrimSpace(payload.FirstName)
	lastName := strings.TrimSpace(payload.LastName)
	birthDateRaw := strings.TrimSpace(payload.BirthDate)
	birthPlace := strings.TrimSpace(payload.BirthPlace)

	if firstName == "" || lastName == "" || birthDateRaw == "" || birthPlace == "" {
		return studentPayload{}, time.Time{}, ErrInvalidInput
	}
	if payload.CompanyID != nil && *payload.CompanyID <= 0 {
		return studentPayload{}, time.Time{}, ErrInvalidInput
	}

	birthDate, err := time.Parse(response.DateFormat, birthDateRaw)
	if err != nil {
		return studentPayload{}, time.Time{}, ErrInvalidInput
	}

	payload.FirstName = firstName
	payload.LastName = lastName
	payload.BirthDate = birthDateRaw
	payload.BirthPlace = birthPlace

	return payload, birthDate, nil
}

func mapStudentGetRow(row dbsqlc.GetStudentByIDRow) StudentDetailsDTO {
	dto := StudentDetailsDTO{
		ID:            row.ID,
		FirstName:     row.Firstname,
		LastName:      row.Lastname,
		SecondName:    pgutil.NullableString(row.Secondname),
		BirthDate:     row.Birthdate.Time.Format(response.DateFormat),
		BirthPlace:    row.Birthplace,
		Pesel:         pgutil.NullableString(row.Pesel),
		AddressStreet: pgutil.NullableString(row.Addressstreet),
		AddressCity:   pgutil.NullableString(row.Addresscity),
		AddressZip:    pgutil.NullableString(row.Addresszip),
		Telephone:     pgutil.NullableString(row.Telephoneno),
		ExternalID:    pgutil.NullableString(row.ExternalID),
	}
	if row.CompanyID.Valid && row.CompanyName.Valid {
		dto.Company = &CompanyDTO{ID: row.CompanyID.Int64, Name: row.CompanyName.String}
	}
	return dto
}
