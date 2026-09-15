package students

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

type fakeServiceDB struct {
	queryRow func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (f fakeServiceDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec call")
}

func (f fakeServiceDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query call")
}

func (f fakeServiceDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if f.queryRow == nil {
		return fakeServiceRow{err: errors.New("unexpected query row call")}
	}
	return f.queryRow(ctx, sql, args...)
}

type fakeServiceRow struct {
	scan func(dest ...interface{}) error
	err  error
}

func (r fakeServiceRow) Scan(dest ...interface{}) error {
	if r.scan != nil {
		return r.scan(dest...)
	}
	return r.err
}

func TestServiceCreateRecordsAuditLog(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), dbsqlc.User{ID: 1, Email: "admin@example.com", Firstname: "Admin", Lastname: "User"})
	txCallCount := 0
	auditRecorded := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTxFn: func(context.Context) (txScope, error) {
			return txScope{
				queries: dbsqlc.New(fakeServiceDB{queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
					// Sprawdzenie duplikatu osoby nie jest częścią scenariusza tych testów.
					if strings.Contains(sql, "-- name: FindDuplicateStudent") {
						return fakeServiceRow{err: pgx.ErrNoRows}
					}
					txCallCount++
					switch txCallCount {
					case 1:
						if !strings.Contains(sql, "INSERT INTO students") {
							return fakeServiceRow{err: errors.New("unexpected create student query")}
						}
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 21
							*(dest[1].(*string)) = "Jan"
							*(dest[2].(*string)) = "Nowak"
							*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
							*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
							*(dest[5].(*string)) = "Warszawa"
							*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
							*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Koszykowa 1", Valid: true}
							*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "Warszawa", Valid: true}
							*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "00-001", Valid: true}
							*(dest[10].(*pgtype.Text)) = pgtype.Text{}
							*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 8, Valid: true}
							*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
							return nil
						}}
					case 2:
						var after StudentDetailsDTO
						if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
							return fakeServiceRow{err: err}
						}
						if args[0] != "student" || args[1] != int64(21) || args[2] != "create" || after.FirstName != "Jan" {
							return fakeServiceRow{err: errors.New("unexpected audit payload")}
						}
						auditRecorded = true
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 60
							return nil
						}}
					default:
						return fakeServiceRow{err: errors.New("unexpected query row call")}
					}
				}}),
				commit:   func(context.Context) error { return nil },
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	created, err := service.Create(ctx, CreateStudentRequest{studentPayload: studentPayload{
		FirstName:     "Jan",
		LastName:      "Nowak",
		SecondName:    ptrString("Adam"),
		BirthDate:     "1990-01-10",
		BirthPlace:    "Warszawa",
		Pesel:         ptrString("90011012345"),
		AddressStreet: ptrString("Koszykowa 1"),
		AddressCity:   ptrString("Warszawa"),
		AddressZip:    ptrString("00-001"),
		CompanyID:     ptrInt64(8),
	}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID != 21 {
		t.Fatalf("expected student id 21, got %d", created.ID)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
}

func TestServiceUpdateRecordsAuditLog(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), dbsqlc.User{ID: 1, Email: "admin@example.com", Firstname: "Admin", Lastname: "User"})
	txCallCount := 0
	auditRecorded := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTxFn: func(context.Context) (txScope, error) {
			return txScope{
				queries: dbsqlc.New(fakeServiceDB{queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
					// Sprawdzenie duplikatu osoby nie jest częścią scenariusza tych testów.
					if strings.Contains(sql, "-- name: FindDuplicateStudent") {
						return fakeServiceRow{err: pgx.ErrNoRows}
					}
					txCallCount++
					switch txCallCount {
					case 1:
						if !strings.Contains(sql, "FROM students s") {
							return fakeServiceRow{err: errors.New("unexpected get student query")}
						}
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 21
							*(dest[1].(*string)) = "Jan"
							*(dest[2].(*string)) = "Nowak"
							*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
							*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
							*(dest[5].(*string)) = "Warszawa"
							*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
							*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Koszykowa 1", Valid: true}
							*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "Warszawa", Valid: true}
							*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "00-001", Valid: true}
							*(dest[10].(*pgtype.Text)) = pgtype.Text{}
							*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 8, Valid: true}
							*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
							return nil
						}}
					case 2:
						if !strings.Contains(sql, "UPDATE students AS s") {
							return fakeServiceRow{err: errors.New("unexpected update student query")}
						}
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 21
							*(dest[1].(*string)) = "Janusz"
							*(dest[2].(*string)) = "Nowak"
							*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
							*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
							*(dest[5].(*string)) = "Warszawa"
							*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
							*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Koszykowa 2", Valid: true}
							*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "Warszawa", Valid: true}
							*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "00-002", Valid: true}
							*(dest[10].(*pgtype.Text)) = pgtype.Text{String: "123456789", Valid: true}
							*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 8, Valid: true}
							*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
							return nil
						}}
					case 3:
						var before StudentDetailsDTO
						if err := json.Unmarshal(args[7].([]byte), &before); err != nil {
							return fakeServiceRow{err: err}
						}
						var after StudentDetailsDTO
						if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
							return fakeServiceRow{err: err}
						}
						if args[2] != "update" || before.FirstName != "Jan" || after.FirstName != "Janusz" {
							return fakeServiceRow{err: errors.New("unexpected audit payload")}
						}
						auditRecorded = true
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 61
							return nil
						}}
					default:
						return fakeServiceRow{err: errors.New("unexpected query row call")}
					}
				}}),
				commit:   func(context.Context) error { return nil },
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	updated, err := service.Update(ctx, 21, UpdateStudentRequest{studentPayload: studentPayload{
		FirstName:     "Janusz",
		LastName:      "Nowak",
		SecondName:    ptrString("Adam"),
		BirthDate:     "1990-01-10",
		BirthPlace:    "Warszawa",
		Pesel:         ptrString("90011012345"),
		AddressStreet: ptrString("Koszykowa 2"),
		AddressCity:   ptrString("Warszawa"),
		AddressZip:    ptrString("00-002"),
		Telephone:     ptrString("123456789"),
		CompanyID:     ptrInt64(8),
	}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.FirstName != "Janusz" {
		t.Fatalf("unexpected updated student: %+v", updated)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
}

func ptrInt64(value int64) *int64 {
	return &value
}

func TestCompanyNotFoundAsMapsOnlyTheCompanyForeignKey(t *testing.T) {
	// Obie nazwy występują w prawdziwych bazach: fk_company ze starego schematu
	// i students_company_id_fkey z migracji 0013.
	for _, name := range []string{"students_company_id_fkey", "fk_company"} {
		fkViolation := &pgconn.PgError{Code: "23503", ConstraintName: name}
		if err := companyNotFoundAs(fkViolation); !errors.Is(err, ErrCompanyNotFound) {
			t.Fatalf("expected ErrCompanyNotFound for %q, got %v", name, err)
		}
	}

	otherConstraint := &pgconn.PgError{Code: "23503", ConstraintName: "some_other_fkey"}
	if err := companyNotFoundAs(otherConstraint); errors.Is(err, ErrCompanyNotFound) {
		t.Fatal("a different foreign key must not be reported as a missing company")
	}

	dbErr := errors.New("connection reset")
	if err := companyNotFoundAs(dbErr); !errors.Is(err, dbErr) {
		t.Fatalf("expected unrelated errors to pass through, got %v", err)
	}
}

func TestStudentWriteErrorMapsPersonUniqueConstraints(t *testing.T) {
	for _, name := range []string{"unique_user_lastname_birthdate", "students_person_unique", "students_person_normalized_uidx"} {
		err := studentWriteError(&pgconn.PgError{Code: "23505", ConstraintName: name})
		if !errors.Is(err, ErrDuplicateStudent) {
			t.Fatalf("expected ErrDuplicateStudent for %q, got %v", name, err)
		}
	}
	if err := studentWriteError(&pgconn.PgError{Code: "23503", ConstraintName: "fk_company"}); !errors.Is(err, ErrCompanyNotFound) {
		t.Fatalf("expected foreign key violations to still map to ErrCompanyNotFound, got %v", err)
	}
	if err := studentWriteError(&pgconn.PgError{Code: "23505", ConstraintName: "students_pkey"}); errors.Is(err, ErrDuplicateStudent) {
		t.Fatal("an unrelated unique violation must not be reported as a duplicate student")
	}
}

func TestPersonKeyChanged(t *testing.T) {
	before := dbsqlc.GetStudentByIDRow{
		Firstname: " Jan",
		Lastname:  "Kowalski ",
		Birthdate: pgtype.Date{Time: time.Date(1990, 1, 10, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	same := func(firstname, lastname string, birthdate time.Time) dbsqlc.UpdateStudentParams {
		return dbsqlc.UpdateStudentParams{Firstname: firstname, Lastname: lastname, Birthdate: pgtype.Date{Time: birthdate, Valid: true}}
	}
	day := time.Date(1990, 1, 10, 0, 0, 0, 0, time.UTC)

	if personKeyChanged(before, same("JAN", "kowalski", day)) {
		t.Fatal("a change of letter case or surrounding spaces must not count as a different person")
	}
	if !personKeyChanged(before, same("Janusz", "Kowalski", day)) {
		t.Fatal("a different first name must count as a change")
	}
	if !personKeyChanged(before, same("Jan", "Kowalski", day.AddDate(0, 0, 1))) {
		t.Fatal("a different birth date must count as a change")
	}
}

// studentWriteDB symuluje bazę dla zapisu kursanta: duplicateID > 0 oznacza, że
// FindDuplicateStudent znajduje innego kursanta; writes zlicza próby zapisu.
type studentWriteDB struct {
	duplicateID int64
	lookups     *int
	writes      *int
}

func (d studentWriteDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (d studentWriteDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (d studentWriteDB) QueryRow(_ context.Context, sql string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(sql, "-- name: FindDuplicateStudent"):
		*d.lookups++
		if d.duplicateID > 0 {
			return fakeServiceRow{scan: func(dest ...interface{}) error {
				*(dest[0].(*int64)) = d.duplicateID
				return nil
			}}
		}
		return fakeServiceRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, "-- name: GetStudentByID"):
		return fakeServiceRow{scan: func(dest ...interface{}) error {
			*(dest[0].(*int64)) = 21
			*(dest[1].(*string)) = "Jan"
			*(dest[2].(*string)) = "Kowalski"
			*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, 1, 10, 0, 0, 0, 0, time.UTC), Valid: true}
			*(dest[5].(*string)) = "Warszawa"
			return nil
		}}
	case strings.Contains(sql, "INSERT INTO students"), strings.Contains(sql, "UPDATE students"):
		*d.writes++
		return fakeServiceRow{err: errors.New("write reached the database")}
	default:
		return fakeServiceRow{err: errors.New("unexpected query row: " + sql)}
	}
}

func studentWriteService(db studentWriteDB) *Service {
	return &Service{
		beginTxFn: func(context.Context) (txScope, error) {
			return txScope{
				queries:  dbsqlc.New(db),
				commit:   func(context.Context) error { return nil },
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}
}

func TestServiceCreateRejectsDuplicatePersonBeforeWriting(t *testing.T) {
	lookups, writes := 0, 0
	service := studentWriteService(studentWriteDB{duplicateID: 7, lookups: &lookups, writes: &writes})

	_, err := service.Create(context.Background(), CreateStudentRequest{studentPayload: studentPayload{
		FirstName: "JAN", LastName: "kowalski", BirthDate: "1990-01-10", BirthPlace: "Warszawa",
	}})

	if !errors.Is(err, ErrDuplicateStudent) {
		t.Fatalf("expected ErrDuplicateStudent, got %v", err)
	}
	if lookups != 1 || writes != 0 {
		t.Fatalf("expected one lookup and no write, got lookups=%d writes=%d", lookups, writes)
	}
}

func TestServiceUpdateSkipsDuplicateCheckWhenPersonIsUnchanged(t *testing.T) {
	// Istniejący duplikat nie może blokować edycji innych danych, np. telefonu.
	lookups, writes := 0, 0
	service := studentWriteService(studentWriteDB{duplicateID: 7, lookups: &lookups, writes: &writes})
	telephone := "500600700"

	_, _ = service.Update(context.Background(), 21, UpdateStudentRequest{studentPayload: studentPayload{
		FirstName: "Jan", LastName: "Kowalski", BirthDate: "1990-01-10", BirthPlace: "Warszawa", Telephone: &telephone,
	}})

	if lookups != 0 {
		t.Fatalf("expected no duplicate lookup when name and birth date are unchanged, got %d", lookups)
	}
	if writes != 1 {
		t.Fatalf("expected the update to reach the database, got %d writes", writes)
	}
}

func TestServiceUpdateRejectsRenameIntoDuplicate(t *testing.T) {
	lookups, writes := 0, 0
	service := studentWriteService(studentWriteDB{duplicateID: 7, lookups: &lookups, writes: &writes})

	_, err := service.Update(context.Background(), 21, UpdateStudentRequest{studentPayload: studentPayload{
		FirstName: "Anna", LastName: "Kowalska", BirthDate: "1990-01-10", BirthPlace: "Warszawa",
	}})

	if !errors.Is(err, ErrDuplicateStudent) {
		t.Fatalf("expected ErrDuplicateStudent, got %v", err)
	}
	if writes != 0 {
		t.Fatalf("expected no write for a duplicate, got %d", writes)
	}
}
