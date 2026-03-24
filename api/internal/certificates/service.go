package certificates

import (
	"context"
	"errors"
	"log"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
)

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrInvalidRegistryDate = errors.New("invalid registry chronology")
)

type CreateCertificateInput struct {
	StudentID       int64
	CourseID        int64
	CertificateDate string
	CourseDateStart string
	CourseDateEnd   *string
	RegistryYear    int64
	RegistryNumber  int32
}

type CreateCertificateResult struct {
	ID int64
}

type txScope struct {
	queries  *dbsqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	pool    *pgxpool.Pool
	queries *dbsqlc.Queries
	beginTx func(context.Context) (txScope, error)
}

func NewService(pool *pgxpool.Pool, queries *dbsqlc.Queries) *Service {
	return &Service{
		pool:    pool,
		queries: queries,
		beginTx: func(ctx context.Context) (txScope, error) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return txScope{}, err
			}
			return newTxScope(tx, queries), nil
		},
	}
}

func (s *Service) Create(ctx context.Context, input CreateCertificateInput) (CreateCertificateResult, error) {
	if err := validateCreateInput(input); err != nil {
		return CreateCertificateResult{}, err
	}

	certificateDate, err := parseDate(input.CertificateDate)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	courseDateStart, err := parseDate(input.CourseDateStart)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	courseDateEnd, err := parseOptionalDate(input.CourseDateEnd)
	if err != nil {
		return CreateCertificateResult{}, err
	}

	if courseDateEnd.Valid && courseDateEnd.Time.Before(courseDateStart.Time) {
		return CreateCertificateResult{}, ErrInvalidInput
	}

	if input.StudentID > math.MaxInt32 {
		return CreateCertificateResult{}, ErrInvalidInput
	}

	rows, err := s.queries.ListRegistryDatesForCourseYear(ctx, dbsqlc.ListRegistryDatesForCourseYearParams{
		CourseID: input.CourseID,
		Year:     input.RegistryYear,
	})
	if err != nil {
		return CreateCertificateResult{}, err
	}

	if err := validateRegistryChronology(rows, input.RegistryNumber, certificateDate); err != nil {
		return CreateCertificateResult{}, err
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return CreateCertificateResult{}, err
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

	registryID, err := tx.queries.CreateRegistry(ctx, dbsqlc.CreateRegistryParams{
		CourseID: input.CourseID,
		Year:     input.RegistryYear,
		Number:   input.RegistryNumber,
	})
	if err != nil {
		return CreateCertificateResult{}, err
	}

	certificateID, err := tx.queries.CreateCertificate(ctx, dbsqlc.CreateCertificateParams{
		Date:            certificateDate,
		StudentID:       validation.Int64ToInt32(input.StudentID),
		Coursedatestart: courseDateStart,
		Coursedateend:   courseDateEnd,
		RegistryID:      registryID,
	})
	if err != nil {
		return CreateCertificateResult{}, err
	}

	if err := tx.commit(ctx); err != nil {
		return CreateCertificateResult{}, err
	}
	committed = true

	return CreateCertificateResult{ID: certificateID}, nil

}

func newTxScope(tx pgx.Tx, queries *dbsqlc.Queries) txScope {
	return txScope{
		queries:  queries.WithTx(tx),
		commit:   tx.Commit,
		rollback: tx.Rollback,
	}
}

func parseDate(value string) (pgtype.Date, error) {
	date, err := time.Parse(time.DateOnly, strings.TrimSpace(value))
	if err != nil {
		return pgtype.Date{}, ErrInvalidInput
	}
	return pgtype.Date{
		Time:             date,
		InfinityModifier: 0,
		Valid:            true,
	}, nil
}

func parseOptionalDate(value *string) (pgtype.Date, error) {
	if value == nil {
		return pgtype.Date{
			Time:             time.Time{},
			InfinityModifier: 0,
			Valid:            false,
		}, nil
	}
	return parseDate(*value)
}

func validateCreateInput(input CreateCertificateInput) error {
	if input.StudentID <= 0 ||
		input.CourseID <= 0 ||
		input.RegistryYear <= 0 ||
		input.RegistryNumber <= 0 ||
		strings.TrimSpace(input.CertificateDate) == "" ||
		strings.TrimSpace(input.CourseDateStart) == "" {
		return ErrInvalidInput
	}
	return nil
}

func validateRegistryChronology(
	rows []dbsqlc.ListRegistryDatesForCourseYearRow,
	registryNumber int32,
	certificateDate pgtype.Date,
) error {
	if !certificateDate.Valid {
		return ErrInvalidInput
	}

	if len(rows) == 0 {
		return nil
	}

	minIdx := -1
	maxIdx := len(rows)

	for i, row := range rows {
		if row.RegistryNumber <= registryNumber && minIdx <= i {
			minIdx = i
		}
		if row.RegistryNumber >= registryNumber && maxIdx >= i {
			maxIdx = i
		}
	}

	minDate := certificateDate.Time
	maxDate := certificateDate.Time

	if minIdx >= 0 {
		minDate = rows[minIdx].CertificateDate.Time
	}
	if maxIdx < len(rows) {
		maxDate = rows[maxIdx].CertificateDate.Time
	}

	if certificateDate.Time.Before(minDate) || certificateDate.Time.After(maxDate) {
		return ErrInvalidRegistryDate
	}

	return nil
}
