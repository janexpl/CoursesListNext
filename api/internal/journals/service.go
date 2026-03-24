package journals

import (
	"context"
	"errors"
	"log"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
)

var (
	ErrJournalAttendeeNotFound          = errors.New("journal attendee not found")
	ErrJournalAttendeeCertificateLinked = errors.New("journal attendee certificate already linked")
	ErrJournalCertificateGeneration     = errors.New("journal certificate generation failed")
)

type GenerateAttendeeCertificateResult struct {
	CertificateID int64
}

type CertificateGenerator interface {
	GenerateAttendeeCertificate(ctx context.Context, journalID, attendeeID int64) (GenerateAttendeeCertificateResult, error)
}

type serviceTxScope struct {
	queries  *sqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
	beginTx func(context.Context) (serviceTxScope, error)
}

func NewService(pool *pgxpool.Pool, queries *sqlc.Queries) *Service {
	return &Service{
		pool:    pool,
		queries: queries,
		beginTx: func(ctx context.Context) (serviceTxScope, error) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return serviceTxScope{}, err
			}

			return serviceTxScope{
				queries:  queries.WithTx(tx),
				commit:   tx.Commit,
				rollback: tx.Rollback,
			}, nil
		},
	}
}

func (s *Service) GenerateAttendeeCertificate(ctx context.Context, journalID, attendeeID int64) (GenerateAttendeeCertificateResult, error) {
	if journalID <= 0 || attendeeID <= 0 {
		return GenerateAttendeeCertificateResult{}, ErrJournalCertificateGeneration
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return GenerateAttendeeCertificateResult{}, err
	}

	committed := false
	defer func() {
		if !committed {
			err = tx.rollback(ctx)
			if err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	source, err := tx.queries.GetJournalAttendeeForCertificateGeneration(ctx, sqlc.GetJournalAttendeeForCertificateGenerationParams{
		JournalID: journalID,
		ID:        attendeeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GenerateAttendeeCertificateResult{}, ErrJournalAttendeeNotFound
		}
		return GenerateAttendeeCertificateResult{}, err
	}

	if source.CertificateID.Valid {
		return GenerateAttendeeCertificateResult{}, ErrJournalAttendeeCertificateLinked
	}

	if source.StudentID > math.MaxInt32 {
		return GenerateAttendeeCertificateResult{}, ErrJournalCertificateGeneration
	}

	registryYear := int64(source.DateEnd.Time.Year())
	rows, err := tx.queries.ListRegistryDatesForCourseYear(ctx, sqlc.ListRegistryDatesForCourseYearParams{
		CourseID: source.CourseID,
		Year:     registryYear,
	})
	if err != nil {
		return GenerateAttendeeCertificateResult{}, err
	}

	certificateDate := source.DateEnd
	registryNumber := int32(1)
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		if last.RegistryNumber >= math.MaxInt32 {
			return GenerateAttendeeCertificateResult{}, ErrJournalCertificateGeneration
		}

		registryNumber = last.RegistryNumber + 1
		if certificateDate.Time.Before(last.CertificateDate.Time) {
			certificateDate = last.CertificateDate
		}
	}

	registryID, err := tx.queries.CreateRegistry(ctx, sqlc.CreateRegistryParams{
		CourseID: source.CourseID,
		Year:     registryYear,
		Number:   registryNumber,
	})
	if err != nil {
		return GenerateAttendeeCertificateResult{}, err
	}

	certificateID, err := tx.queries.CreateCertificate(ctx, sqlc.CreateCertificateParams{
		Date:            certificateDate,
		StudentID:       validation.Int64ToInt32(source.StudentID),
		Coursedatestart: source.DateStart,
		Coursedateend:   pgtype.Date{Time: source.DateEnd.Time, Valid: true},
		RegistryID:      registryID,
	})
	if err != nil {
		return GenerateAttendeeCertificateResult{}, err
	}

	_, err = tx.queries.UpdateJournalAttendeeCertificate(ctx, sqlc.UpdateJournalAttendeeCertificateParams{
		JournalID:     journalID,
		AttendeeID:    attendeeID,
		CertificateID: pgtype.Int8{Int64: certificateID, Valid: true},
	})
	if err != nil {
		return GenerateAttendeeCertificateResult{}, err
	}

	if err := tx.commit(ctx); err != nil {
		return GenerateAttendeeCertificateResult{}, err
	}
	committed = true

	return GenerateAttendeeCertificateResult{CertificateID: certificateID}, nil
}
