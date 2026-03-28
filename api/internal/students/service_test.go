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
