package companies

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
	commitCalled := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTxFn: func(context.Context) (txScope, error) {
			return txScope{
				queries: dbsqlc.New(fakeServiceDB{queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
					txCallCount++
					switch txCallCount {
					case 1:
						if !strings.Contains(sql, "INSERT INTO companies") {
							return fakeServiceRow{err: errors.New("unexpected create company query")}
						}
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 15
							*(dest[1].(*string)) = "ABC Sp. z o.o."
							*(dest[2].(*string)) = "Koszykowa 1"
							*(dest[3].(*string)) = "Warszawa"
							*(dest[4].(*string)) = "00-001"
							*(dest[5].(*string)) = "1234567890"
							*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "biuro@abc.pl", Valid: true}
							*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Jan Nowak", Valid: true}
							*(dest[8].(*string)) = "500600700"
							*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "Kluczowy klient", Valid: true}
							return nil
						}}
					case 2:
						if !strings.Contains(sql, "INSERT INTO audit_log") {
							return fakeServiceRow{err: errors.New("unexpected audit query")}
						}
						var after CompanyDetailsDTO
						if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
							return fakeServiceRow{err: err}
						}
						if args[0] != "company" || args[1] != int64(15) || args[2] != "create" || after.Name != "ABC Sp. z o.o." {
							return fakeServiceRow{err: errors.New("unexpected audit payload")}
						}
						auditRecorded = true
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 50
							return nil
						}}
					default:
						return fakeServiceRow{err: errors.New("unexpected query row call")}
					}
				}}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	created, err := service.Create(ctx, CreateCompanyRequest{
		Name:          "ABC Sp. z o.o.",
		Street:        "Koszykowa 1",
		City:          "Warszawa",
		Zipcode:       "00-001",
		Nip:           "1234567890",
		Email:         ptrString("biuro@abc.pl"),
		ContactPerson: ptrString("Jan Nowak"),
		Telephone:     "500600700",
		Note:          ptrString("Kluczowy klient"),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created.ID != 15 {
		t.Fatalf("expected company id 15, got %d", created.ID)
	}
	if !auditRecorded || !commitCalled {
		t.Fatal("expected audit and commit for company create")
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
						if !strings.Contains(sql, "FROM companies") {
							return fakeServiceRow{err: errors.New("unexpected get company query")}
						}
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 15
							*(dest[1].(*string)) = "ABC Sp. z o.o."
							*(dest[2].(*string)) = "Koszykowa 1"
							*(dest[3].(*string)) = "Warszawa"
							*(dest[4].(*string)) = "00-001"
							*(dest[5].(*string)) = "1234567890"
							*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "biuro@abc.pl", Valid: true}
							*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Jan Nowak", Valid: true}
							*(dest[8].(*string)) = "500600700"
							*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "Kluczowy klient", Valid: true}
							return nil
						}}
					case 2:
						if !strings.Contains(sql, "UPDATE companies") {
							return fakeServiceRow{err: errors.New("unexpected update company query")}
						}
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 15
							*(dest[1].(*string)) = "ABC Sp. z o.o. po zmianie"
							*(dest[2].(*string)) = "Koszykowa 2"
							*(dest[3].(*string)) = "Warszawa"
							*(dest[4].(*string)) = "00-001"
							*(dest[5].(*string)) = "1234567890"
							*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "biuro@abc.pl", Valid: true}
							*(dest[7].(*pgtype.Text)) = pgtype.Text{String: "Jan Nowak", Valid: true}
							*(dest[8].(*string)) = "500600700"
							*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "Kluczowy klient", Valid: true}
							return nil
						}}
					case 3:
						var before CompanyDetailsDTO
						if err := json.Unmarshal(args[7].([]byte), &before); err != nil {
							return fakeServiceRow{err: err}
						}
						var after CompanyDetailsDTO
						if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
							return fakeServiceRow{err: err}
						}
						if args[2] != "update" || before.Name != "ABC Sp. z o.o." || after.Name != "ABC Sp. z o.o. po zmianie" {
							return fakeServiceRow{err: errors.New("unexpected audit payload")}
						}
						auditRecorded = true
						return fakeServiceRow{scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 51
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

	updated, err := service.Update(ctx, 15, UpdateCompanyDTO{
		Name:          "ABC Sp. z o.o. po zmianie",
		Street:        "Koszykowa 2",
		City:          "Warszawa",
		Zipcode:       "00-001",
		Nip:           "1234567890",
		Email:         ptrString("biuro@abc.pl"),
		ContactPerson: ptrString("Jan Nowak"),
		Telephone:     "500600700",
		Note:          ptrString("Kluczowy klient"),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "ABC Sp. z o.o. po zmianie" {
		t.Fatalf("unexpected updated company: %+v", updated)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
}
