package courses

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
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

type fakeServiceDB struct {
	exec     func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	query    func(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	queryRow func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (f fakeServiceDB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if f.exec == nil {
		return pgconn.CommandTag{}, errors.New("unexpected exec call")
	}
	return f.exec(ctx, sql, args...)
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

func TestServiceUpdateReturnsNotFoundWhenCourseDoesNotExist(t *testing.T) {
	rollbackCalled := false
	commitCalled := false

	service := &Service{
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: sqlc.New(fakeServiceDB{
					queryRow: func(_ context.Context, sql string, _ ...interface{}) pgx.Row {
						if !strings.Contains(sql, "FROM courses") {
							t.Fatalf("unexpected query: %s", sql)
						}
						return fakeServiceRow{err: pgx.ErrNoRows}
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

	_, err := service.Update(context.Background(), 12, UpdateCourseInput{
		MainName:      "BHP",
		Name:          "Szkolenie okresowe",
		Symbol:        "BHP-OKR",
		ExpiryTime:    "5",
		CourseProgram: `[{"Subject":"Intro"}]`,
		CertFrontPage: "<p>Front</p>",
	})

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
	if !rollbackCalled {
		t.Fatal("expected rollback to be called")
	}
	if commitCalled {
		t.Fatal("did not expect commit to be called")
	}
}

func TestNormalizeTranslationInputRejectsUnsupportedLanguageCode(t *testing.T) {
	_, err := normalizeTranslationInput([]CourseTranslationInput{
		{
			LanguageCode:  "fr",
			CourseName:    "Formation",
			CourseProgram: `[{"Subject":"Introduction"}]`,
			CertFrontPage: "<p>Front</p>",
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestServiceUpdateRecordsAuditLogWithBeforeAndAfter(t *testing.T) {
	ctx := auth.ContextWithUser(context.Background(), sqlc.User{
		ID:        9,
		Email:     "jan@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
	})

	translationsListCallCount := 0
	auditRecorded := false
	commitCalled := false
	rollbackCalled := false

	service := &Service{
		recorder: auditlog.NewRecorder(),
		beginTx: func(context.Context) (txScope, error) {
			return txScope{
				queries: sqlc.New(fakeServiceDB{
					queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
						switch {
						case strings.Contains(sql, "SELECT id, mainname, name, symbol"):
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 12
								*(dest[1].(*pgtype.Text)) = pgtype.Text{String: "BHP", Valid: true}
								*(dest[2].(*string)) = "Szkolenie okresowe"
								*(dest[3].(*string)) = "BHP-OKR"
								*(dest[4].(*pgtype.Text)) = pgtype.Text{String: "5", Valid: true}
								*(dest[5].(*[]byte)) = []byte(`[{"Subject":"Intro"}]`)
								*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "<p>Front</p>", Valid: true}
								return nil
							}}
						case strings.Contains(sql, "UPDATE courses"):
							if len(args) != 7 || args[0] != int64(12) {
								t.Fatalf("unexpected update args: %+v", args)
							}
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 12
								*(dest[1].(*pgtype.Text)) = pgtype.Text{String: "BHP", Valid: true}
								*(dest[2].(*string)) = "Szkolenie okresowe z audytem"
								*(dest[3].(*string)) = "BHP-OKR"
								*(dest[4].(*pgtype.Text)) = pgtype.Text{String: "3", Valid: true}
								*(dest[5].(*[]byte)) = []byte(`[{"Subject":"Safety"}]`)
								*(dest[6].(*pgtype.Text)) = pgtype.Text{String: "<p>Front EN</p>", Valid: true}
								return nil
							}}
						case strings.Contains(sql, "INSERT INTO course_certificate_translations"):
							return fakeServiceRow{scan: func(dest ...interface{}) error {
								*(dest[0].(*int64)) = 1
								*(dest[1].(*int64)) = 12
								*(dest[2].(*string)) = "en"
								*(dest[3].(*string)) = "Periodic training"
								*(dest[4].(*string)) = `[{"Subject":"Introduction"}]`
								*(dest[5].(*string)) = "<p>Front EN</p>"
								*(dest[6].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
								*(dest[7].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
								return nil
							}}
						case strings.Contains(sql, "INSERT INTO audit_log"):
							if len(args) != 10 {
								t.Fatalf("expected 10 audit args, got %d", len(args))
							}
							if args[0] != "course" || args[1] != int64(12) || args[2] != "update" {
								t.Fatalf("unexpected audit args prefix: %+v", args[:3])
							}

							var before CourseDetailDTO
							if err := json.Unmarshal(args[7].([]byte), &before); err != nil {
								t.Fatalf("failed to unmarshal before audit payload: %v", err)
							}
							var after CourseDetailDTO
							if err := json.Unmarshal(args[8].([]byte), &after); err != nil {
								t.Fatalf("failed to unmarshal after audit payload: %v", err)
							}

							if before.Name != "Szkolenie okresowe" || before.ExpiryTime == nil || *before.ExpiryTime != "5" {
								t.Fatalf("unexpected before audit payload: %+v", before)
							}
							if after.Name != "Szkolenie okresowe z audytem" || after.ExpiryTime == nil || *after.ExpiryTime != "3" {
								t.Fatalf("unexpected after audit payload: %+v", after)
							}
							if len(after.CertificateTranslations) != 1 || after.CertificateTranslations[0].LanguageCode != "en" {
								t.Fatalf("unexpected audit translations payload: %+v", after.CertificateTranslations)
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
					query: func(_ context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
						if !strings.Contains(sql, "FROM course_certificate_translations") {
							return nil, errors.New("unexpected query call")
						}
						translationsListCallCount++
						return &fakeServiceRows{scans: []func(dest ...any) error{
							func(dest ...any) error {
								*(dest[0].(*int64)) = 1
								*(dest[1].(*int64)) = 12
								*(dest[2].(*string)) = "en"
								*(dest[3].(*string)) = "Periodic training"
								*(dest[4].(*string)) = `[{"Subject":"Introduction"}]`
								*(dest[5].(*string)) = "<p>Front EN</p>"
								*(dest[6].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
								*(dest[7].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
								return nil
							},
						}}, nil
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

	updated, err := service.Update(ctx, 12, UpdateCourseInput{
		MainName:      "BHP",
		Name:          "Szkolenie okresowe z audytem",
		Symbol:        "BHP-OKR",
		ExpiryTime:    "3",
		CourseProgram: `[{"Subject":"Safety"}]`,
		CertFrontPage: "<p>Front EN</p>",
		CertificateTranslations: []CourseTranslationInput{
			{
				LanguageCode:  "en",
				CourseName:    "Periodic training",
				CourseProgram: `[{"Subject":"Introduction"}]`,
				CertFrontPage: "<p>Front EN</p>",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "Szkolenie okresowe z audytem" {
		t.Fatalf("unexpected updated course payload: %+v", updated)
	}
	if !auditRecorded {
		t.Fatal("expected audit log to be recorded")
	}
	if translationsListCallCount != 3 {
		t.Fatalf("expected 3 translation list calls, got %d", translationsListCallCount)
	}
	if !commitCalled {
		t.Fatal("expected commit to be called")
	}
	if rollbackCalled {
		t.Fatal("did not expect rollback after successful commit")
	}
}

func TestSyncCourseCertificateTranslationsDeletesMissingTranslations(t *testing.T) {
	upsertedLanguages := make([]string, 0, 1)
	deletedLanguages := make([]string, 0, 1)

	queries := sqlc.New(fakeServiceDB{
		queryRow: func(_ context.Context, sql string, args ...interface{}) pgx.Row {
			if !strings.Contains(sql, "INSERT INTO course_certificate_translations") {
				t.Fatalf("unexpected query row sql: %s", sql)
			}
			upsertedLanguages = append(upsertedLanguages, args[1].(string))
			return fakeServiceRow{scan: func(dest ...interface{}) error {
				*(dest[0].(*int64)) = 1
				*(dest[1].(*int64)) = args[0].(int64)
				*(dest[2].(*string)) = args[1].(string)
				*(dest[3].(*string)) = args[2].(string)
				*(dest[4].(*string)) = string(args[3].([]byte))
				*(dest[5].(*string)) = args[4].(string)
				*(dest[6].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
				*(dest[7].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
				return nil
			}}
		},
		query: func(_ context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
			if !strings.Contains(sql, "FROM course_certificate_translations") {
				t.Fatalf("unexpected query sql: %s", sql)
			}
			if len(args) != 1 || args[0] != int64(7) {
				t.Fatalf("unexpected list args: %+v", args)
			}
			return &fakeServiceRows{
				scans: []func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*int64)) = 11
						*(dest[1].(*int64)) = 7
						*(dest[2].(*string)) = "de"
						*(dest[3].(*string)) = "Deutsch"
						*(dest[4].(*string)) = "[]"
						*(dest[5].(*string)) = "<p>DE</p>"
						*(dest[6].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
						*(dest[7].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
						return nil
					},
					func(dest ...any) error {
						*(dest[0].(*int64)) = 12
						*(dest[1].(*int64)) = 7
						*(dest[2].(*string)) = "en"
						*(dest[3].(*string)) = "English"
						*(dest[4].(*string)) = "[]"
						*(dest[5].(*string)) = "<p>EN</p>"
						*(dest[6].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
						*(dest[7].(*pgtype.Timestamptz)) = pgtype.Timestamptz{}
						return nil
					},
				},
			}, nil
		},
		exec: func(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
			if !strings.Contains(sql, "DELETE FROM course_certificate_translations") {
				t.Fatalf("unexpected exec sql: %s", sql)
			}
			if len(args) != 2 || args[0] != int64(7) {
				t.Fatalf("unexpected delete args: %+v", args)
			}
			deletedLanguages = append(deletedLanguages, args[1].(string))
			return pgconn.CommandTag{}, nil
		},
	})

	err := syncCourseCertificateTranslations(context.Background(), queries, 7, []CourseTranslationInput{
		{
			LanguageCode:  "en",
			CourseName:    "English",
			CourseProgram: "[]",
			CertFrontPage: "<p>EN</p>",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(upsertedLanguages) != 1 || upsertedLanguages[0] != "en" {
		t.Fatalf("expected only en upsert, got %+v", upsertedLanguages)
	}
	if len(deletedLanguages) != 1 || deletedLanguages[0] != "de" {
		t.Fatalf("expected only de deletion, got %+v", deletedLanguages)
	}
}
