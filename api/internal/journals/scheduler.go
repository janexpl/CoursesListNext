package journals

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

// ErrProgramExceedsJournalDates oznacza, że sesje wygenerowane z programu kursu
// (maks. 8 godzin dziennie) nie mieszczą się między dateStart a dateEnd dziennika.
var ErrProgramExceedsJournalDates = errors.New("course program does not fit within journal dates")

// SessionScheduler zapisuje dziennik lub jego sesje wygenerowane z programu kursu
// i pilnuje, żeby żadna sesja nie wypadła po dacie zakończenia dziennika.
type SessionScheduler interface {
	CreateJournal(ctx context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error)
	GenerateSessionsFromCourse(ctx context.Context, journalID int64, dateEnd pgtype.Date) (int64, error)
}

// Router przekazuje handlerowi *Service; ta asercja gwarantuje w czasie kompilacji,
// że handler dostanie wersję z kontrolą dat, a nie zapis wprost przez querier.
var _ SessionScheduler = (*Service)(nil)

// CreateJournal tworzy dziennik razem z sesjami i wycofuje całość, jeśli ostatnia
// sesja wypada po dateEnd. Rozkład sesji zostaje w jednym miejscu (SQL CreateJournal),
// a tutaj sprawdzamy tylko jego wynik.
func (s *Service) CreateJournal(ctx context.Context, arg sqlc.CreateJournalParams) (sqlc.CreateJournalRow, error) {
	var created sqlc.CreateJournalRow
	err := s.withSessionsWithinDates(ctx, arg.DateEnd, func(q *sqlc.Queries) (int64, error) {
		row, err := q.CreateJournal(ctx, arg)
		if err != nil {
			return 0, err
		}
		created = row
		return row.ID, nil
	})
	return created, err
}

// GenerateSessionsFromCourse generuje sesje dla istniejącego dziennika z tą samą
// kontrolą dat co przy tworzeniu.
func (s *Service) GenerateSessionsFromCourse(ctx context.Context, journalID int64, dateEnd pgtype.Date) (int64, error) {
	var generated int64
	err := s.withSessionsWithinDates(ctx, dateEnd, func(q *sqlc.Queries) (int64, error) {
		count, err := q.GenerateJournalSessionsFromCourse(ctx, journalID)
		if err != nil {
			return 0, err
		}
		generated = count
		return journalID, nil
	})
	return generated, err
}

// withSessionsWithinDates wykonuje zapis w transakcji i zatwierdza go tylko wtedy,
// gdy ostatnia sesja dziennika mieści się w dateEnd. write zwraca id dziennika.
func (s *Service) withSessionsWithinDates(ctx context.Context, dateEnd pgtype.Date, write func(*sqlc.Queries) (int64, error)) error {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.rollback(ctx); err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	journalID, err := write(tx.queries)
	if err != nil {
		return err
	}

	lastSession, err := tx.queries.GetJournalLastSessionDate(ctx, journalID)
	if err != nil {
		return err
	}
	if lastSession.Valid && dateEnd.Valid && lastSession.Time.After(dateEnd.Time) {
		return ErrProgramExceedsJournalDates
	}

	if err := tx.commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}
