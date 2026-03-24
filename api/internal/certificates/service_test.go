package certificates

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
