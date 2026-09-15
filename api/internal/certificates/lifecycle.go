package certificates

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

var (
	// ErrCertificateNotFound - zaświadczenie nie istnieje albo zostało usunięte.
	ErrCertificateNotFound = errors.New("certificate not found")
	// ErrCertificateAlreadyRevoked - ponowne unieważnienie.
	ErrCertificateAlreadyRevoked = errors.New("certificate already revoked")
	// ErrCertificateRevoked - operacja niedozwolona na unieważnionym dokumencie
	// (duplikat, edycja, PDF).
	ErrCertificateRevoked = errors.New("certificate is revoked")
	// ErrCertificateAlreadySuperseded - dokument ma już (nieusunięty) duplikat.
	ErrCertificateAlreadySuperseded = errors.New("certificate already superseded")
)

const certificateSupersedesUniqueIndex = "certificates_supersedes_id_uidx"

// Revoke unieważnia zaświadczenie. Dokument zostaje w bazie i na listach, a jego numer
// rejestru pozostaje zajęty (patrz zapytania w registries.sql).
func (s *Service) Revoke(ctx context.Context, certificateID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if certificateID <= 0 || reason == "" {
		return ErrInvalidInput
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	if _, err := tx.queries.LockCertificateForLifecycle(ctx, certificateID); err != nil {
		return notFoundAs(err, ErrCertificateNotFound)
	}
	state, err := tx.queries.GetCertificateLifecycleState(ctx, certificateID)
	if err != nil {
		return err
	}
	if state.Revoked {
		return ErrCertificateAlreadyRevoked
	}

	before, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return notFoundAs(err, ErrCertificateNotFound)
	}

	revokedBy := pgtype.Int8{}
	if user, ok := auth.UserFromContext(ctx); ok {
		revokedBy = pgtype.Int8{Int64: user.ID, Valid: true}
	}
	if err := tx.queries.RevokeCertificate(ctx, dbsqlc.RevokeCertificateParams{
		ID:              certificateID,
		Reason:          reason,
		RevokedByUserID: revokedBy,
	}); err != nil {
		return err
	}

	if s.recorder != nil {
		after, err := tx.queries.GetCertificateByID(ctx, certificateID)
		if err != nil {
			return err
		}
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "certificate",
			EntityID:   certificateID,
			Action:     "update",
			Before:     mapCertificateDetailsResponse(before, nil),
			After:      mapCertificateDetailsResponse(after, nil),
			Metadata:   map[string]any{"operation": "revoke", "reason": reason},
		}); err != nil {
			return err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

// Duplicate wystawia nowy dokument z danymi oryginału, z dzisiejszą datą i numerem
// rejestru nadanym jak przy wystawieniu bez numeru: rok z daty zakończenia kursu (albo
// rozpoczęcia), numer = największy w kursie i roku + 1. Zwraca id nowego dokumentu.
func (s *Service) Duplicate(ctx context.Context, originalID int64, reason string) (int64, error) {
	return s.duplicate(ctx, originalID, reason, time.Now())
}

func (s *Service) duplicate(ctx context.Context, originalID int64, reason string, now time.Time) (int64, error) {
	reason = strings.TrimSpace(reason)
	if originalID <= 0 || reason == "" {
		return 0, ErrInvalidInput
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	original, err := tx.queries.LockCertificateForLifecycle(ctx, originalID)
	if err != nil {
		return 0, notFoundAs(err, ErrCertificateNotFound)
	}
	state, err := tx.queries.GetCertificateLifecycleState(ctx, originalID)
	if err != nil {
		return 0, err
	}
	if state.Revoked {
		return 0, ErrCertificateRevoked
	}
	if state.Superseded {
		return 0, ErrCertificateAlreadySuperseded
	}

	registryYear := int64(original.CourseDateStart.Time.Year())
	if original.CourseDateEnd.Valid {
		registryYear = int64(original.CourseDateEnd.Time.Year())
	}
	issueDate := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}

	if err := tx.queries.AcquireRegistryLock(ctx, dbsqlc.AcquireRegistryLockParams{
		CourseID: strconv.FormatInt(original.CourseID, 10),
		Year:     strconv.FormatInt(registryYear, 10),
	}); err != nil {
		return 0, err
	}
	registryNumber, err := tx.queries.GetNextRegistryNumber(ctx, dbsqlc.GetNextRegistryNumberParams{
		CourseID: original.CourseID,
		Year:     registryYear,
	})
	if err != nil {
		return 0, err
	}
	rows, err := tx.queries.ListRegistryDatesForCourseYear(ctx, dbsqlc.ListRegistryDatesForCourseYearParams{
		CourseID: original.CourseID,
		Year:     registryYear,
	})
	if err != nil {
		return 0, err
	}
	if err := validateRegistryChronology(rows, registryNumber, issueDate); err != nil {
		return 0, err
	}
	registryID, err := tx.queries.CreateRegistry(ctx, dbsqlc.CreateRegistryParams{
		CourseID: original.CourseID,
		Year:     registryYear,
		Number:   registryNumber,
	})
	if err != nil {
		return 0, err
	}

	duplicateID, err := tx.queries.DuplicateCertificate(ctx, dbsqlc.DuplicateCertificateParams{
		Date:       issueDate,
		RegistryID: registryID,
		Reason:     reason,
		OriginalID: originalID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == certificateSupersedesUniqueIndex {
			return 0, ErrCertificateAlreadySuperseded
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrCertificateNotFound
		}
		return 0, err
	}

	if s.recorder != nil {
		created, err := tx.queries.GetCertificateByID(ctx, duplicateID)
		if err != nil {
			return 0, err
		}
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "certificate",
			EntityID:   duplicateID,
			Action:     "create",
			After:      mapCertificateDetailsResponse(created, nil),
			Metadata:   map[string]any{"source": "duplicate", "supersedesId": originalID, "reason": reason},
		}); err != nil {
			return 0, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return 0, err
	}
	committed = true
	return duplicateID, nil
}
