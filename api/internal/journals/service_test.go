package journals

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

type fakeServiceDB struct {
	query    func(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	queryRow func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (f fakeServiceDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec call")
}

func (f fakeServiceDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if f.query == nil {
		return nil, errors.New("unexpected query call")
	}
	return f.query(ctx, sql, args...)
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

type fakeServiceRows struct {
	index int
	scans []func(dest ...any) error
	err   error
}

func (r *fakeServiceRows) Close() {}

func (r *fakeServiceRows) Err() error {
	return r.err
}

func (r *fakeServiceRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeServiceRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeServiceRows) Next() bool {
	if r.index >= len(r.scans) {
		return false
	}
	r.index++
	return true
}

func (r *fakeServiceRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.scans) {
		return errors.New("scan called without current row")
	}
	return r.scans[r.index-1](dest...)
}

func (r *fakeServiceRows) Values() ([]any, error) {
	return nil, nil
}

func (r *fakeServiceRows) RawValues() [][]byte {
	return nil
}

func (r *fakeServiceRows) Conn() *pgx.Conn {
	return nil
}

func scanGeneratedCertificateDetailsRow(dest ...interface{}) error {
	*(dest[0].(*int64)) = 101
	*(dest[1].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true}
	*(dest[2].(*int32)) = 12
	*(dest[3].(*string)) = "Jan"
	*(dest[4].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
	*(dest[5].(*string)) = "Nowak"
	*(dest[6].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
	*(dest[7].(*string)) = "Warszawa"
	*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
	*(dest[9].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
	*(dest[10].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true}
	*(dest[11].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true}
	*(dest[12].(*int64)) = 77
	*(dest[13].(*int64)) = 2026
	*(dest[14].(*int64)) = 1
	*(dest[15].(*int64)) = 3
	*(dest[16].(*string)) = "Szkolenie BHP"
	*(dest[17].(*string)) = "BHP"
	*(dest[18].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
	*(dest[19].(*string)) = `{"sections":["intro"]}`
	*(dest[20].(*string)) = "<p>Front</p>"
	*(dest[21].(*string)) = "pl"
	*(dest[22].(*pgtype.Int8)) = pgtype.Int8{Int64: 7, Valid: true}
	*(dest[23].(*pgtype.Int8)) = pgtype.Int8{Int64: 21, Valid: true}
	*(dest[24].(*pgtype.Text)) = pgtype.Text{String: "Szkolenie okresowe", Valid: true}
	*(dest[25].(*pgtype.Text)) = pgtype.Text{String: "closed", Valid: true}
	*(dest[26].(*interface{})) = "2029-03-15"
	return nil
}

func TestGenerateAttendeeCertificateCreatesPolishSnapshot(t *testing.T) {
	registryRows := &fakeServiceRows{}
	txCallCount := 0
	commitCalled := false
	rollbackCalled := false

	service := &Service{
		queries: sqlc.New(fakeServiceDB{}),
		beginTx: func(context.Context) (serviceTxScope, error) {
			return serviceTxScope{
				queries: sqlc.New(fakeServiceDB{
					query: func(_ context.Context, _ string, args ...interface{}) (pgx.Rows, error) {
						if len(args) != 2 {
							t.Fatalf("expected 2 query args, got %d", len(args))
						}
						return registryRows, nil
					},
					queryRow: func(_ context.Context, _ string, args ...interface{}) pgx.Row {
						txCallCount++
						switch txCallCount {
						case 1:
							if len(args) != 2 || args[0] != int64(21) || args[1] != int64(7) {
								t.Fatalf("unexpected attendee source args: %+v", args)
							}
							return fakeServiceRow{
								scan: func(dest ...interface{}) error {
									*(dest[0].(*int64)) = 7
									*(dest[1].(*int64)) = 21
									*(dest[2].(*int64)) = 12
									*(dest[3].(*pgtype.Int8)) = pgtype.Int8{}
									*(dest[4].(*int64)) = 3
									*(dest[5].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true}
									*(dest[6].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true}
									*(dest[7].(*string)) = "Jan"
									*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
									*(dest[9].(*string)) = "Nowak"
									*(dest[10].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
									*(dest[11].(*string)) = "Warszawa"
									*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
									*(dest[13].(*pgtype.Int8)) = pgtype.Int8{Int64: 4, Valid: true}
									*(dest[14].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
									*(dest[15].(*string)) = "Szkolenie BHP"
									*(dest[16].(*string)) = "BHP"
									*(dest[17].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
									*(dest[18].(*string)) = `{"sections":["intro"]}`
									*(dest[19].(*pgtype.Text)) = pgtype.Text{String: "<p>Front</p>", Valid: true}
									return nil
								},
							}
						case 2:
							if len(args) != 3 || args[0] != int64(3) || args[1] != int64(2026) || args[2] != int32(1) {
								t.Fatalf("unexpected create registry args: %+v", args)
							}
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 77
								return nil
							}}
						case 3:
							if len(args) != 19 {
								t.Fatalf("expected 19 create certificate args, got %d", len(args))
							}
							if args[5] != "pl" || args[6] != "Jan" || args[8] != "Nowak" || args[14] != "Szkolenie BHP" {
								t.Fatalf("unexpected create certificate args: %+v", args)
							}
							companyIDSnapshot, ok := args[13].(pgtype.Int8)
							if !ok || !companyIDSnapshot.Valid || companyIDSnapshot.Int64 != 4 {
								t.Fatalf("expected company_id_snapshot=4, got %+v", args[13])
							}
							if args[18] != "<p>Front</p>" {
								t.Fatalf("unexpected front page snapshot: %+v", args[18])
							}
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 101
								return nil
							}}
						case 4:
							if len(args) != 3 {
								t.Fatalf("unexpected update attendee args: %+v", args)
							}
							certificateID, ok := args[2].(pgtype.Int8)
							if !ok || !certificateID.Valid || certificateID.Int64 != 101 {
								t.Fatalf("unexpected linked certificate id: %+v", args[2])
							}
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 7
								*(dest[1].(*int64)) = 21
								*(dest[2].(*int64)) = 12
								*(dest[3].(*pgtype.Int8)) = pgtype.Int8{Int64: 101, Valid: true}
								*(dest[4].(*string)) = "Jan Nowak"
								*(dest[5].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
								*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
								*(dest[7].(*int32)) = 1
								*(dest[8].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
								*(dest[9].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true}
								*(dest[10].(*pgtype.Int8)) = pgtype.Int8{Int64: 2026, Valid: true}
								*(dest[11].(*interface{})) = int64(1)
								*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "BHP", Valid: true}
								return nil
							}}
						default:
							return fakeServiceRow{err: errors.New("unexpected query row call")}
						}
					},
				}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error {
					rollbackCalled = true
					return nil
				},
			}, nil
		},
	}

	result, err := service.GenerateAttendeeCertificate(context.Background(), 21, 7)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CertificateID != 101 {
		t.Fatalf("expected certificate id 101, got %d", result.CertificateID)
	}
	if !commitCalled {
		t.Fatal("expected commit to be called")
	}
	if rollbackCalled {
		t.Fatal("did not expect rollback after successful commit")
	}
}

func TestGenerateAttendeeCertificateRecordsAuditLog(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), sqlc.User{
		ID:        9,
		Email:     "jan@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
	})

	registryRows := &fakeServiceRows{}
	txCallCount := 0
	auditRecorded := false
	commitCalled := false
	rollbackCalled := false

	service := &Service{
		queries:  sqlc.New(fakeServiceDB{}),
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (serviceTxScope, error) {
			return serviceTxScope{
				queries: sqlc.New(fakeServiceDB{
					query: func(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
						return registryRows, nil
					},
					queryRow: func(_ context.Context, _ string, args ...interface{}) pgx.Row {
						txCallCount++
						switch txCallCount {
						case 1:
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 7
								*(dest[1].(*int64)) = 21
								*(dest[2].(*int64)) = 12
								*(dest[3].(*pgtype.Int8)) = pgtype.Int8{}
								*(dest[4].(*int64)) = 3
								*(dest[5].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true}
								*(dest[6].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true}
								*(dest[7].(*string)) = "Jan"
								*(dest[8].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
								*(dest[9].(*string)) = "Nowak"
								*(dest[10].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
								*(dest[11].(*string)) = "Warszawa"
								*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
								*(dest[13].(*pgtype.Int8)) = pgtype.Int8{Int64: 4, Valid: true}
								*(dest[14].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
								*(dest[15].(*string)) = "Szkolenie BHP"
								*(dest[16].(*string)) = "BHP"
								*(dest[17].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
								*(dest[18].(*string)) = `{"sections":["intro"]}`
								*(dest[19].(*pgtype.Text)) = pgtype.Text{String: "<p>Front</p>", Valid: true}
								return nil
							}}
						case 2:
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 77
								return nil
							}}
						case 3:
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 101
								return nil
							}}
						case 4:
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 7
								*(dest[1].(*int64)) = 21
								*(dest[2].(*int64)) = 12
								*(dest[3].(*pgtype.Int8)) = pgtype.Int8{Int64: 101, Valid: true}
								*(dest[4].(*string)) = "Jan Nowak"
								*(dest[5].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
								*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
								*(dest[7].(*int32)) = 1
								*(dest[8].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
								*(dest[9].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true}
								*(dest[10].(*pgtype.Int8)) = pgtype.Int8{Int64: 2026, Valid: true}
								*(dest[11].(*interface{})) = int64(1)
								*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "BHP", Valid: true}
								return nil
							}}
						case 5:
							return fakeServiceRow{scan: scanGeneratedCertificateDetailsRow}
						case 6:
							if len(args) != 10 {
								return fakeServiceRow{err: errors.New("unexpected audit args count")}
							}
							if args[0] != "certificate" || args[1] != int64(101) || args[2] != "create" {
								return fakeServiceRow{err: errors.New("unexpected audit args prefix")}
							}

							var after map[string]any
							if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
								return fakeServiceRow{err: err}
							}
							var metadata map[string]any
							if err := json.Unmarshal(args[9].([]byte), &metadata); err != nil {
								return fakeServiceRow{err: err}
							}

							if after["courseName"] != "Szkolenie BHP" || after["languageCode"] != "pl" {
								return fakeServiceRow{err: errors.New("unexpected audit after payload")}
							}
							if metadata["source"] != "journal" {
								return fakeServiceRow{err: errors.New("unexpected audit metadata")}
							}
							auditRecorded = true

							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 55
								return nil
							}}
						default:
							return fakeServiceRow{err: errors.New("unexpected query row call")}
						}
					},
				}),
				commit: func(context.Context) error {
					commitCalled = true
					return nil
				},
				rollback: func(context.Context) error {
					rollbackCalled = true
					return nil
				},
			}, nil
		},
	}

	result, err := service.GenerateAttendeeCertificate(ctx, 21, 7)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CertificateID != 101 {
		t.Fatalf("expected certificate id 101, got %d", result.CertificateID)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
	if !commitCalled {
		t.Fatal("expected commit to be called")
	}
	if rollbackCalled {
		t.Fatal("did not expect rollback after successful commit")
	}
}

func TestBuildJournalCertificateParamsRejectsInvalidSnapshotSource(t *testing.T) {
	_, err := buildJournalCertificateParams(sqlc.GetJournalAttendeeForCertificateGenerationRow{})
	if !errors.Is(err, ErrJournalCertificateGeneration) {
		t.Fatalf("expected ErrJournalCertificateGeneration, got %v", err)
	}
}
