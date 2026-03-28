package companies

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
)

var ErrInvalidInput = errors.New("invalid input")

type txScope struct {
	queries  *dbsqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	queries   *dbsqlc.Queries
	recorder  *auditlog.Recorder
	beginTxFn func(context.Context) (txScope, error)
}

func NewService(pool *pgxpool.Pool, queries *dbsqlc.Queries, recorder *auditlog.Recorder) *Service {
	return &Service{
		queries:  queries,
		recorder: recorder,
		beginTxFn: func(ctx context.Context) (txScope, error) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return txScope{}, err
			}

			return txScope{
				queries:  queries.WithTx(tx),
				commit:   tx.Commit,
				rollback: tx.Rollback,
			}, nil
		},
	}
}

func (s *Service) Create(ctx context.Context, req CreateCompanyRequest) (CompanyDetailsDTO, error) {
	params, err := buildCreateCompanyParams(req)
	if err != nil {
		return CompanyDetailsDTO{}, err
	}

	tx, err := s.beginTxFn(ctx)
	if err != nil {
		return CompanyDetailsDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	createdCompany, err := tx.queries.CreateCompany(ctx, params)
	if err != nil {
		return CompanyDetailsDTO{}, err
	}

	createdSnapshot := mapCompanyDetailRow(createdCompany)
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "company",
			EntityID:   createdCompany.ID,
			Action:     "create",
			Before:     nil,
			After:      createdSnapshot,
			Metadata:   nil,
		}); err != nil {
			return CompanyDetailsDTO{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return CompanyDetailsDTO{}, err
	}
	committed = true

	return createdSnapshot, nil
}

func (s *Service) Update(ctx context.Context, companyID int64, req UpdateCompanyDTO) (CompanyDetailsDTO, error) {
	if companyID <= 0 {
		return CompanyDetailsDTO{}, ErrInvalidInput
	}

	params, err := buildUpdateCompanyParams(companyID, req)
	if err != nil {
		return CompanyDetailsDTO{}, err
	}

	tx, err := s.beginTxFn(ctx)
	if err != nil {
		return CompanyDetailsDTO{}, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	beforeCompany, err := tx.queries.GetCompanyByID(ctx, companyID)
	if err != nil {
		return CompanyDetailsDTO{}, err
	}

	updatedCompany, err := tx.queries.UpdateCompany(ctx, params)
	if err != nil {
		return CompanyDetailsDTO{}, err
	}

	beforeSnapshot := mapCompanyDetailRow(beforeCompany)
	afterSnapshot := mapCompanyDetailRow(updatedCompany)
	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
			EntityType: "company",
			EntityID:   companyID,
			Action:     "update",
			Before:     beforeSnapshot,
			After:      afterSnapshot,
			Metadata:   nil,
		}); err != nil {
			return CompanyDetailsDTO{}, err
		}
	}

	if err := tx.commit(ctx); err != nil {
		return CompanyDetailsDTO{}, err
	}
	committed = true

	return afterSnapshot, nil
}

func buildCreateCompanyParams(req CreateCompanyRequest) (dbsqlc.CreateCompanyParams, error) {
	name := strings.TrimSpace(req.Name)
	street := strings.TrimSpace(req.Street)
	city := strings.TrimSpace(req.City)
	zipcode := strings.TrimSpace(req.Zipcode)
	nip := strings.TrimSpace(req.Nip)
	telephone := strings.TrimSpace(req.Telephone)

	if name == "" || street == "" || city == "" || zipcode == "" || nip == "" || telephone == "" {
		return dbsqlc.CreateCompanyParams{}, ErrInvalidInput
	}

	return dbsqlc.CreateCompanyParams{
		Name:          name,
		Street:        street,
		City:          city,
		Zipcode:       zipcode,
		Nip:           nip,
		Email:         pgutil.OptionalText(req.Email),
		Contactperson: pgutil.OptionalText(req.ContactPerson),
		Telephoneno:   telephone,
		Note:          pgutil.OptionalText(req.Note),
	}, nil
}

func buildUpdateCompanyParams(companyID int64, req UpdateCompanyDTO) (dbsqlc.UpdateCompanyParams, error) {
	params, err := buildCreateCompanyParams(CreateCompanyRequest(req))
	if err != nil {
		return dbsqlc.UpdateCompanyParams{}, err
	}

	return dbsqlc.UpdateCompanyParams{
		ID:            companyID,
		Name:          params.Name,
		Street:        params.Street,
		City:          params.City,
		Zipcode:       params.Zipcode,
		Nip:           params.Nip,
		Email:         params.Email,
		Contactperson: params.Contactperson,
		Telephoneno:   params.Telephoneno,
		Note:          params.Note,
	}, nil
}
