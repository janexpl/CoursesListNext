package certificates

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

func scanCertificateDetailsRow(dest ...interface{}) error {
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
	*(dest[14].(*int64)) = 18
	*(dest[15].(*int64)) = 3
	*(dest[16].(*string)) = "Szkolenie BHP"
	*(dest[17].(*string)) = "BHP"
	*(dest[18].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
	*(dest[19].(*string)) = `{"sections":["intro"]}`
	*(dest[20].(*string)) = "<p>Front</p>"
	*(dest[21].(*string)) = "pl"
	*(dest[22].(*pgtype.Int8)) = pgtype.Int8{}
	*(dest[23].(*pgtype.Int8)) = pgtype.Int8{}
	*(dest[24].(*pgtype.Text)) = pgtype.Text{}
	*(dest[25].(*pgtype.Text)) = pgtype.Text{}
	*(dest[26].(*interface{})) = "2029-03-15"
	return nil
}

func TestCreateReturnsInvalidInputForMissingRequiredFields(t *testing.T) {
	service := &Service{}

	_, err := service.Create(context.Background(), CreateCertificateInput{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateReturnsInvalidInputForInvalidDate(t *testing.T) {
	service := &Service{}

	_, err := service.Create(context.Background(), CreateCertificateInput{
		StudentID:       1,
		CourseID:        2,
		CertificateDate: "2026-99-99",
		CourseDateStart: "2026-03-10",
		RegistryYear:    2026,
		RegistryNumber:  10,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateReturnsInvalidRegistryDateWhenChronologyDoesNotMatch(t *testing.T) {
	rows := &fakeServiceRows{
		scans: []func(dest ...any) error{
			func(dest ...any) error {
				*(dest[0].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[1].(*int32)) = 1
				return nil
			},
			func(dest ...any) error {
				*(dest[0].(*pgtype.Date)) = pgtype.Date{Time: time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC), Valid: true}
				*(dest[1].(*int32)) = 3
				return nil
			},
		},
	}

	service := &Service{
		queries: dbsqlc.New(fakeServiceDB{
			query: func(_ context.Context, _ string, args ...interface{}) (pgx.Rows, error) {
				if len(args) != 2 {
					t.Fatalf("expected 2 query args, got %d", len(args))
				}
				return rows, nil
			},
		}),
		beginTx: func(context.Context) (txScope, error) {
			t.Fatal("transaction should not start for invalid chronology")
			return txScope{}, nil
		},
	}

	_, err := service.Create(context.Background(), CreateCertificateInput{
		StudentID:       1,
		CourseID:        2,
		CertificateDate: "2026-03-25",
		CourseDateStart: "2026-03-24",
		RegistryYear:    2026,
		RegistryNumber:  2,
	})
	if !errors.Is(err, ErrInvalidRegistryDate) {
		t.Fatalf("expected ErrInvalidRegistryDate, got %v", err)
	}
}

func TestCreateReturnsCertificateIDOnSuccess(t *testing.T) {
	readRows := &fakeServiceRows{}
	txCallCount := 0
	baseQueryRowCount := 0
	commitCalled := false
	rollbackCalled := false

	service := &Service{
		queries: dbsqlc.New(fakeServiceDB{
			query: func(_ context.Context, _ string, args ...interface{}) (pgx.Rows, error) {
				if len(args) != 2 {
					t.Fatalf("expected 2 query args, got %d", len(args))
				}
				return readRows, nil
			},
			queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
				baseQueryRowCount++
				switch {
				case strings.Contains(sql, "FROM students s"):
					return fakeServiceRow{
						scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 12
							*(dest[1].(*string)) = "Jan"
							*(dest[2].(*string)) = "Nowak"
							*(dest[3].(*pgtype.Text)) = pgtype.Text{}
							*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
							*(dest[5].(*string)) = "Warszawa"
							*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
							*(dest[7].(*pgtype.Text)) = pgtype.Text{}
							*(dest[8].(*pgtype.Text)) = pgtype.Text{}
							*(dest[9].(*pgtype.Text)) = pgtype.Text{}
							*(dest[10].(*pgtype.Text)) = pgtype.Text{}
							*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 3, Valid: true}
							*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
							return nil
						},
					}
				case strings.Contains(sql, "FROM courses"):
					return fakeServiceRow{
						scan: func(dest ...interface{}) error {
							*(dest[0].(*int64)) = 3
							*(dest[1].(*pgtype.Text)) = pgtype.Text{String: "Szkolenie", Valid: true}
							*(dest[2].(*string)) = "Szkolenie BHP"
							*(dest[3].(*string)) = "BHP"
							*(dest[4].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
							*(dest[5].(*[]byte)) = []byte(`{"sections":["intro"]}`)
							*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "<p>Front</p>", Valid: true}
							return nil
						},
					}
				default:
					return fakeServiceRow{err: errors.New("unexpected base query row call")}
				}
			},
		}),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: dbsqlc.New(fakeServiceDB{
					queryRow: func(_ context.Context, _ string, _ ...interface{}) pgx.Row {
						txCallCount++
						switch txCallCount {
						case 1:
							return fakeServiceRow{
								scan: func(dest ...interface{}) error {
									*(dest[0].(*int64)) = 77
									return nil
								},
							}
						case 2:
							return fakeServiceRow{
								scan: func(dest ...interface{}) error {
									*(dest[0].(*int64)) = 101
									return nil
								},
							}
						default:
							return fakeServiceRow{err: errors.New("unexpected tx query row call")}
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

	result, err := service.Create(context.Background(), CreateCertificateInput{
		StudentID:       12,
		CourseID:        3,
		CertificateDate: "2026-03-15",
		CourseDateStart: "2026-03-10",
		RegistryYear:    2026,
		RegistryNumber:  18,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != 101 {
		t.Fatalf("expected certificate id 101, got %d", result.ID)
	}
	if !commitCalled {
		t.Fatal("expected commit to be called")
	}
	if rollbackCalled {
		t.Fatal("did not expect rollback after successful commit")
	}
	if baseQueryRowCount != 2 {
		t.Fatalf("expected 2 base query row calls, got %d", baseQueryRowCount)
	}
}

func TestCreateRecordsAuditLogWithCreatedCertificateSnapshot(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), dbsqlc.User{
		ID:        9,
		Email:     "jan@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
	})

	readRows := &fakeServiceRows{}
	txCallCount := 0
	auditRecorded := false

	service := &Service{
		queries: dbsqlc.New(fakeServiceDB{
			query: func(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
				return readRows, nil
			},
			queryRow: func(_ context.Context, sql string, _ ...interface{}) pgx.Row {
				switch {
				case strings.Contains(sql, "FROM students s"):
					return fakeServiceRow{scan: func(dest ...interface{}) error {
						*(dest[0].(*int64)) = 12
						*(dest[1].(*string)) = "Jan"
						*(dest[2].(*string)) = "Nowak"
						*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
						*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
						*(dest[5].(*string)) = "Warszawa"
						*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
						*(dest[7].(*pgtype.Text)) = pgtype.Text{}
						*(dest[8].(*pgtype.Text)) = pgtype.Text{}
						*(dest[9].(*pgtype.Text)) = pgtype.Text{}
						*(dest[10].(*pgtype.Text)) = pgtype.Text{}
						*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 3, Valid: true}
						*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
						return nil
					}}
				case strings.Contains(sql, "FROM courses"):
					return fakeServiceRow{scan: func(dest ...interface{}) error {
						*(dest[0].(*int64)) = 3
						*(dest[1].(*pgtype.Text)) = pgtype.Text{String: "Szkolenie", Valid: true}
						*(dest[2].(*string)) = "Szkolenie BHP"
						*(dest[3].(*string)) = "BHP"
						*(dest[4].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
						*(dest[5].(*[]byte)) = []byte(`{"sections":["intro"]}`)
						*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "<p>Front</p>", Valid: true}
						return nil
					}}
				default:
					return fakeServiceRow{err: errors.New("unexpected base query row call")}
				}
			},
		}),
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: dbsqlc.New(fakeServiceDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						txCallCount++
						switch txCallCount {
						case 1:
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 77
								return nil
							}}
						case 2:
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 101
								return nil
							}}
						case 3:
							if !strings.Contains(sql, "SELECT\n    c.id,") && !strings.Contains(sql, "SELECT c.id,") {
								return fakeServiceRow{scan: scanCertificateDetailsRow}
							}
							return fakeServiceRow{scan: scanCertificateDetailsRow}
						case 4:
							var after CertificateDetailsDTO
							if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
								t.Fatalf("failed to unmarshal audit after payload: %v", err)
							}
							if args[0] != "certificate" || args[1] != int64(101) || args[2] != "create" {
								t.Fatalf("unexpected audit args prefix: %+v", args[:3])
							}
							if after.ID != 101 || after.CourseName != "Szkolenie BHP" || after.LanguageCode != "pl" {
								t.Fatalf("unexpected audit after payload: %+v", after)
							}
							auditRecorded = true
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 55
								return nil
							}}
						default:
							return fakeServiceRow{err: errors.New("unexpected tx query row call")}
						}
					},
				}),
				commit:   func(context.Context) error { return nil },
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	result, err := service.Create(ctx, CreateCertificateInput{
		StudentID:       12,
		CourseID:        3,
		CertificateDate: "2026-03-15",
		CourseDateStart: "2026-03-10",
		RegistryYear:    2026,
		RegistryNumber:  18,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != 101 {
		t.Fatalf("expected certificate id 101, got %d", result.ID)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
}

func TestUpdateReturnsInvalidInputForMissingRequiredFields(t *testing.T) {
	service := &Service{}

	_, err := service.Update(context.Background(), 21, UpdateCertificateInput{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestUpdateRefreshesStudentSnapshotAndReturnsUpdatedCertificate(t *testing.T) {
	baseQueryRowCount := 0
	txQueryRowCount := 0
	service := &Service{
		queries: dbsqlc.New(fakeServiceDB{
			queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
				baseQueryRowCount++
				if !strings.Contains(sql, "FROM students s") {
					return fakeServiceRow{err: errors.New("unexpected base query row call")}
				}
				if len(args) != 1 || args[0] != int64(12) {
					t.Fatalf("expected student lookup for id 12, got %+v", args)
				}
				return fakeServiceRow{
					scan: func(dest ...interface{}) error {
						*(dest[0].(*int64)) = 12
						*(dest[1].(*string)) = "Jan"
						*(dest[2].(*string)) = "Nowak"
						*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
						*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
						*(dest[5].(*string)) = "Warszawa"
						*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
						*(dest[7].(*pgtype.Text)) = pgtype.Text{}
						*(dest[8].(*pgtype.Text)) = pgtype.Text{}
						*(dest[9].(*pgtype.Text)) = pgtype.Text{}
						*(dest[10].(*pgtype.Text)) = pgtype.Text{}
						*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 4, Valid: true}
						*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
						return nil
					},
				}
			},
		}),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: dbsqlc.New(fakeServiceDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						txQueryRowCount++
						switch txQueryRowCount {
						case 1:
							return fakeServiceRow{scan: scanCertificateDetailsRow}
						case 2:
							if len(args) != 12 {
								t.Fatalf("expected 12 update args, got %d", len(args))
							}
							if args[11] != int64(21) {
								t.Fatalf("expected certificate id 21, got %+v", args[11])
							}
							if args[4] != "Jan" || args[6] != "Nowak" || args[8] != "Warszawa" {
								t.Fatalf("unexpected snapshot args: %+v", args)
							}
							return fakeServiceRow{
								scan: func(dest ...interface{}) error {
									*(dest[0].(*int64)) = 21
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
									*(dest[14].(*int64)) = 18
									*(dest[15].(*int64)) = 3
									*(dest[16].(*string)) = "Szkolenie BHP"
									*(dest[17].(*string)) = "BHP"
									*(dest[18].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
									*(dest[19].(*string)) = `{"sections":["intro"]}`
									*(dest[20].(*string)) = "<p>Front</p>"
									*(dest[21].(*string)) = "pl"
									return nil
								},
							}
						default:
							return fakeServiceRow{err: errors.New("unexpected tx query row call")}
						}
					},
				}),
				commit:   func(context.Context) error { return nil },
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	row, err := service.Update(context.Background(), 21, UpdateCertificateInput{
		StudentID:       12,
		CertificateDate: "2026-03-15",
		CourseDateStart: "2026-03-10",
		CourseDateEnd:   ptr("2026-03-15"),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if row.ID != 21 || row.StudentFirstname != "Jan" || row.LanguageCode != "pl" {
		t.Fatalf("unexpected updated row: %+v", row)
	}
	if baseQueryRowCount != 1 {
		t.Fatalf("expected 1 base query row call, got %d", baseQueryRowCount)
	}
	if txQueryRowCount != 2 {
		t.Fatalf("expected 2 tx query row calls, got %d", txQueryRowCount)
	}
}

func TestUpdateRecordsAuditLogWithBeforeAndAfterSnapshots(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), dbsqlc.User{
		ID:        9,
		Email:     "jan@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
	})

	baseQueryRowCount := 0
	txQueryRowCount := 0
	auditRecorded := false

	service := &Service{
		queries: dbsqlc.New(fakeServiceDB{
			queryRow: func(_ context.Context, sql string, _ ...interface{}) pgx.Row {
				baseQueryRowCount++
				if !strings.Contains(sql, "FROM students s") {
					return fakeServiceRow{err: errors.New("unexpected base query row call")}
				}
				return fakeServiceRow{scan: func(dest ...interface{}) error {
					*(dest[0].(*int64)) = 12
					*(dest[1].(*string)) = "Jan"
					*(dest[2].(*string)) = "Nowak"
					*(dest[3].(*pgtype.Text)) = pgtype.Text{String: "Adam", Valid: true}
					*(dest[4].(*pgtype.Date)) = pgtype.Date{Time: time.Date(1990, time.January, 10, 0, 0, 0, 0, time.UTC), Valid: true}
					*(dest[5].(*string)) = "Warszawa"
					*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "90011012345", Valid: true}
					*(dest[7].(*pgtype.Text)) = pgtype.Text{}
					*(dest[8].(*pgtype.Text)) = pgtype.Text{}
					*(dest[9].(*pgtype.Text)) = pgtype.Text{}
					*(dest[10].(*pgtype.Text)) = pgtype.Text{}
					*(dest[11].(*pgtype.Int8)) = pgtype.Int8{Int64: 4, Valid: true}
					*(dest[12].(*pgtype.Text)) = pgtype.Text{String: "ABC Sp. z o.o.", Valid: true}
					return nil
				}}
			},
		}),
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: dbsqlc.New(fakeServiceDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						txQueryRowCount++
						switch txQueryRowCount {
						case 1:
							if !strings.Contains(sql, "SELECT\n    c.id,") && !strings.Contains(sql, "SELECT c.id,") {
								return fakeServiceRow{err: errors.New("unexpected before certificate query")}
							}
							return fakeServiceRow{scan: scanCertificateDetailsRow}
						case 2:
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 21
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
								*(dest[14].(*int64)) = 18
								*(dest[15].(*int64)) = 3
								*(dest[16].(*string)) = "Szkolenie BHP po zmianie"
								*(dest[17].(*string)) = "BHP"
								*(dest[18].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
								*(dest[19].(*string)) = `{"sections":["intro"]}`
								*(dest[20].(*string)) = "<p>Front</p>"
								*(dest[21].(*string)) = "pl"
								return nil
							}}
						case 3:
							var before CertificateDetailsDTO
							if err := json.Unmarshal(args[7].([]byte), &before); err != nil {
								t.Fatalf("failed to unmarshal before audit payload: %v", err)
							}
							var after CertificateDetailsDTO
							if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
								t.Fatalf("failed to unmarshal after audit payload: %v", err)
							}
							if before.ID != 101 || before.CourseName != "Szkolenie BHP" {
								t.Fatalf("unexpected before audit payload: %+v", before)
							}
							if after.ID != 21 || after.CourseName != "Szkolenie BHP po zmianie" {
								t.Fatalf("unexpected after audit payload: %+v", after)
							}
							auditRecorded = true
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 55
								return nil
							}}
						default:
							return fakeServiceRow{err: errors.New("unexpected tx query row call")}
						}
					},
				}),
				commit:   func(context.Context) error { return nil },
				rollback: func(context.Context) error { return nil },
			}, nil
		},
	}

	_, err := service.Update(ctx, 21, UpdateCertificateInput{
		StudentID:       12,
		CertificateDate: "2026-03-15",
		CourseDateStart: "2026-03-10",
		CourseDateEnd:   ptr("2026-03-15"),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if baseQueryRowCount != 1 {
		t.Fatalf("expected 1 base query row call, got %d", baseQueryRowCount)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
}

func ptr(value string) *string {
	return &value
}

func TestValidateRegistryChronologyAllowsDateWithinBounds(t *testing.T) {
	rows := []dbsqlc.ListRegistryDatesForCourseYearRow{
		{
			CertificateDate: pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true},
			RegistryNumber:  1,
		},
		{
			CertificateDate: pgtype.Date{Time: time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC), Valid: true},
			RegistryNumber:  3,
		},
	}

	err := validateRegistryChronology(rows, 2, pgtype.Date{Time: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), Valid: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRegistryChronologyRejectsDateOutsideBounds(t *testing.T) {
	rows := []dbsqlc.ListRegistryDatesForCourseYearRow{
		{
			CertificateDate: pgtype.Date{Time: time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC), Valid: true},
			RegistryNumber:  1,
		},
		{
			CertificateDate: pgtype.Date{Time: time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC), Valid: true},
			RegistryNumber:  3,
		},
	}

	err := validateRegistryChronology(rows, 2, pgtype.Date{Time: time.Date(2026, time.March, 25, 0, 0, 0, 0, time.UTC), Valid: true})
	if !errors.Is(err, ErrInvalidRegistryDate) {
		t.Fatalf("expected ErrInvalidRegistryDate, got %v", err)
	}
}
