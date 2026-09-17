package certificates

import (
	"context"
	"errors"
	"log"
	"strings"

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
)

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
	revoked, err := tx.queries.GetCertificateLifecycleState(ctx, certificateID)
	if err != nil {
		return err
	}
	if revoked {
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

	after, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return err
	}
	if s.publisher != nil {
		if err := s.publisher.CertificateRevoked(ctx, tx.queries, after, reason); err != nil {
			return err
		}
	}
	if s.recorder != nil {
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

// Duplicate odnotowuje wystawienie duplikatu (wtórnika) na istniejącym zaświadczeniu.
// Duplikat to ten sam dokument o tym samym numerze rejestru, z adnotacją "DUPLIKAT"
// i datą na wydruku - nie powstaje nowe zaświadczenie i nie jest zajmowany nowy numer.
// Kolejne wystawienie nadpisuje datę i powód; każde zostaje w historii zmian.
func (s *Service) Duplicate(ctx context.Context, certificateID int64, reason string) error {
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
	revoked, err := tx.queries.GetCertificateLifecycleState(ctx, certificateID)
	if err != nil {
		return err
	}
	if revoked {
		return ErrCertificateRevoked
	}

	before, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return notFoundAs(err, ErrCertificateNotFound)
	}

	issuedBy := pgtype.Int8{}
	if user, ok := auth.UserFromContext(ctx); ok {
		issuedBy = pgtype.Int8{Int64: user.ID, Valid: true}
	}
	if err := tx.queries.MarkCertificateDuplicateIssued(ctx, dbsqlc.MarkCertificateDuplicateIssuedParams{
		ID:                      certificateID,
		Reason:                  reason,
		DuplicateIssuedByUserID: issuedBy,
	}); err != nil {
		return err
	}

	after, err := tx.queries.GetCertificateByID(ctx, certificateID)
	if err != nil {
		return err
	}
	if s.publisher != nil {
		if err := s.publisher.CertificateDuplicateIssued(ctx, tx.queries, after, reason); err != nil {
			return err
		}
	}
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "certificate",
			EntityID:   certificateID,
			Action:     "update",
			Before:     mapCertificateDetailsResponse(before, nil),
			After:      mapCertificateDetailsResponse(after, nil),
			Metadata:   map[string]any{"operation": "duplicate", "reason": reason},
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
