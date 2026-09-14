package journals

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

// schedulerDB symuluje bazę w transakcji: zapis dziennika lub sesji oraz odczyt
// daty ostatniej sesji, który decyduje o zatwierdzeniu.
type schedulerDB struct {
	lastSession pgtype.Date
	writeErr    error
}

func (d schedulerDB) Exec(_ context.Context, sql string, _ ...interface{}) (pgconn.CommandTag, error) {
	if !strings.Contains(sql, "INSERT INTO training_journal_sessions") {
		return pgconn.CommandTag{}, errors.New("unexpected exec: " + sql)
	}
	if d.writeErr != nil {
		return pgconn.CommandTag{}, d.writeErr
	}
	return pgconn.NewCommandTag("INSERT 0 4"), nil
}

func (d schedulerDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (d schedulerDB) QueryRow(_ context.Context, sql string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(sql, "-- name: CreateJournal"):
		if d.writeErr != nil {
			return fakeServiceRow{err: d.writeErr}
		}
		return fakeServiceRow{scan: func(dest ...interface{}) error {
			*(dest[0].(*int64)) = 55
			return nil
		}}
	case strings.Contains(sql, "last_session_date"):
		return fakeServiceRow{scan: func(dest ...interface{}) error {
			*(dest[0].(*pgtype.Date)) = d.lastSession
			return nil
		}}
	default:
		return fakeServiceRow{err: errors.New("unexpected query row: " + sql)}
	}
}

type txSpy struct {
	committed  bool
	rolledBack bool
}

func schedulerService(db schedulerDB, spy *txSpy) *Service {
	return &Service{
		beginTx: func(context.Context) (serviceTxScope, error) {
			return serviceTxScope{
				queries: sqlc.New(db),
				commit: func(context.Context) error {
					spy.committed = true
					return nil
				},
				rollback: func(context.Context) error {
					spy.rolledBack = true
					return nil
				},
			}, nil
		},
	}
}

func date(year int, month time.Month, day int) pgtype.Date {
	return pgtype.Date{Time: time.Date(year, month, day, 0, 0, 0, 0, time.UTC), Valid: true}
}

func TestCreateJournalRollsBackWhenSessionsExceedEndDate(t *testing.T) {
	spy := &txSpy{}
	service := schedulerService(schedulerDB{lastSession: date(2026, 9, 13)}, spy)

	_, err := service.CreateJournal(context.Background(), sqlc.CreateJournalParams{
		CourseID:  128,
		DateStart: date(2026, 9, 10),
		DateEnd:   date(2026, 9, 12),
	})

	if !errors.Is(err, ErrProgramExceedsJournalDates) {
		t.Fatalf("expected ErrProgramExceedsJournalDates, got %v", err)
	}
	if spy.committed {
		t.Fatal("a journal whose sessions exceed its end date must not be committed")
	}
	if !spy.rolledBack {
		t.Fatal("expected the transaction to be rolled back")
	}
}

func TestCreateJournalCommitsWhenSessionsFitDates(t *testing.T) {
	for name, lastSession := range map[string]pgtype.Date{
		"ostatnia sesja w dniu zakończenia": date(2026, 9, 12),
		"ostatnia sesja przed końcem":       date(2026, 9, 11),
		"brak sesji (pusty program)":        {},
	} {
		t.Run(name, func(t *testing.T) {
			spy := &txSpy{}
			service := schedulerService(schedulerDB{lastSession: lastSession}, spy)

			row, err := service.CreateJournal(context.Background(), sqlc.CreateJournalParams{
				CourseID:  128,
				DateStart: date(2026, 9, 10),
				DateEnd:   date(2026, 9, 12),
			})

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if row.ID != 55 {
				t.Fatalf("expected the created journal to be returned, got id %d", row.ID)
			}
			if !spy.committed || spy.rolledBack {
				t.Fatalf("expected commit without rollback, got committed=%v rolledBack=%v", spy.committed, spy.rolledBack)
			}
		})
	}
}

func TestCreateJournalPassesThroughWriteErrors(t *testing.T) {
	// Brak kursu (pgx.ErrNoRows) musi dotrzeć do handlera bez zmian - to on
	// zamienia go na 404 "course not found".
	spy := &txSpy{}
	service := schedulerService(schedulerDB{writeErr: pgx.ErrNoRows}, spy)

	_, err := service.CreateJournal(context.Background(), sqlc.CreateJournalParams{DateEnd: date(2026, 9, 12)})

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
	if spy.committed {
		t.Fatal("a failed write must not be committed")
	}
}

func TestGenerateSessionsFromCourseRollsBackWhenSessionsExceedEndDate(t *testing.T) {
	spy := &txSpy{}
	service := schedulerService(schedulerDB{lastSession: date(2026, 9, 15)}, spy)

	_, err := service.GenerateSessionsFromCourse(context.Background(), 55, date(2026, 9, 12))

	if !errors.Is(err, ErrProgramExceedsJournalDates) {
		t.Fatalf("expected ErrProgramExceedsJournalDates, got %v", err)
	}
	if spy.committed || !spy.rolledBack {
		t.Fatalf("expected rollback without commit, got committed=%v rolledBack=%v", spy.committed, spy.rolledBack)
	}
}

func TestGenerateSessionsFromCourseReturnsCountWhenSessionsFit(t *testing.T) {
	spy := &txSpy{}
	service := schedulerService(schedulerDB{lastSession: date(2026, 9, 12)}, spy)

	count, err := service.GenerateSessionsFromCourse(context.Background(), 55, date(2026, 9, 12))

	if err != nil || count != 4 {
		t.Fatalf("expected 4 generated sessions, got %d (%v)", count, err)
	}
	if !spy.committed {
		t.Fatal("expected the generated sessions to be committed")
	}
}
