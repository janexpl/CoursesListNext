package companies

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/validation"
)

var ErrInvalidInput = errors.New("invalid input")

// ErrInvalidNIP opakowuje konkretny błąd z pakietu validation, żeby handler mógł
// zwrócić ten sam komunikat co GET /companies/lookup-by-nip.
var ErrInvalidNIP = errors.New("invalid nip")

// MaxExternalIDLength to limit długości identyfikatora platformy (CHECK w migracji 0020).
const MaxExternalIDLength = 64

// NaturalKeyConflictError - NIP z PUT by-external-id należy do innej firmy.
// ID to kolidujący rekord; 0, gdy nie udało się go ustalić.
type NaturalKeyConflictError struct {
	ID int64
}

func (e *NaturalKeyConflictError) Error() string {
	return "company with the same natural key already exists"
}

func findCompanyIDByNIP(ctx context.Context, q *dbsqlc.Queries, nip string, excludeID pgtype.Int8) (int64, error) {
	id, err := q.FindCompanyIDByNIP(ctx, dbsqlc.FindCompanyIDByNIPParams{Nip: nip, ExcludeID: excludeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

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

// UpsertByExternalID zakłada firmę o danym identyfikatorze platformy albo nadpisuje
// istniejącą (jak PATCH). created mówi, czy rekord powstał. NIP należący do innej firmy
// kończy się *NaturalKeyConflictError - bez tworzenia duplikatu i bez zmiany cudzego rekordu.
func (s *Service) UpsertByExternalID(ctx context.Context, externalID string, req CreateCompanyRequest) (CompanyDetailsDTO, bool, error) {
	if externalID == "" || len([]rune(externalID)) > MaxExternalIDLength {
		return CompanyDetailsDTO{}, false, ErrInvalidInput
	}
	params, err := buildCreateCompanyParams(req)
	if err != nil {
		return CompanyDetailsDTO{}, false, err
	}
	externalIDText := pgtype.Text{String: externalID, Valid: true}

	company, created, err := s.upsertByExternalID(ctx, externalIDText, params)
	if isNIPUniqueViolation(err) {
		// Wyścig z równoległym zapisem firmy o tym NIP-ie: transakcja jest już wycofana,
		// więc kolidujący rekord szukamy poza nią.
		conflict := &NaturalKeyConflictError{}
		if s.queries != nil {
			excludeID := pgtype.Int8{}
			if existingID, lookupErr := s.queries.GetCompanyIDByExternalID(ctx, externalIDText); lookupErr == nil {
				excludeID = pgtype.Int8{Int64: existingID, Valid: true}
			}
			if id, lookupErr := findCompanyIDByNIP(ctx, s.queries, params.Nip, excludeID); lookupErr == nil {
				conflict.ID = id
			}
		}
		err = conflict
	}
	return company, created, err
}

func (s *Service) upsertByExternalID(ctx context.Context, externalID pgtype.Text, params dbsqlc.CreateCompanyParams) (CompanyDetailsDTO, bool, error) {
	tx, err := s.beginTxFn(ctx)
	if err != nil {
		return CompanyDetailsDTO{}, false, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.rollback(ctx); rollbackErr != nil {
				log.Printf("unable to rollback changes: %v", rollbackErr)
			}
		}
	}()

	if err := tx.queries.AcquireCompanyExternalIDLock(ctx, externalID.String); err != nil {
		return CompanyDetailsDTO{}, false, err
	}

	existingID, err := tx.queries.GetCompanyIDByExternalID(ctx, externalID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		existingID = 0
	case err != nil:
		return CompanyDetailsDTO{}, false, err
	}

	excludeID := pgtype.Int8{}
	if existingID != 0 {
		excludeID = pgtype.Int8{Int64: existingID, Valid: true}
	}
	duplicateID, err := findCompanyIDByNIP(ctx, tx.queries, params.Nip, excludeID)
	if err != nil {
		return CompanyDetailsDTO{}, false, err
	}
	if duplicateID != 0 {
		return CompanyDetailsDTO{}, false, &NaturalKeyConflictError{ID: duplicateID}
	}

	var (
		result CompanyDetailsDTO
		entry  auditlog.Entry
	)
	if existingID == 0 {
		createdCompany, err := tx.queries.CreateCompany(ctx, params)
		if err != nil {
			return CompanyDetailsDTO{}, false, err
		}
		if err := tx.queries.SetCompanyExternalID(ctx, dbsqlc.SetCompanyExternalIDParams{
			ExternalID: externalID,
			ID:         createdCompany.ID,
		}); err != nil {
			return CompanyDetailsDTO{}, false, err
		}
		createdCompany.ExternalID = externalID
		result = mapCompanyDetailRow(createdCompany)
		entry = auditlog.Entry{EntityType: "company", EntityID: createdCompany.ID, Action: "create", After: result}
	} else {
		beforeCompany, err := tx.queries.GetCompanyByID(ctx, existingID)
		if err != nil {
			return CompanyDetailsDTO{}, false, err
		}
		updatedCompany, err := tx.queries.UpdateCompany(ctx, dbsqlc.UpdateCompanyParams{
			ID:                         existingID,
			Name:                       params.Name,
			Street:                     params.Street,
			City:                       params.City,
			Zipcode:                    params.Zipcode,
			Nip:                        params.Nip,
			Email:                      params.Email,
			Contactperson:              params.Contactperson,
			Telephoneno:                params.Telephoneno,
			Note:                       params.Note,
			ExpiryNotificationsEnabled: params.ExpiryNotificationsEnabled,
			ExpiryNotificationEmail:    params.ExpiryNotificationEmail,
		})
		if err != nil {
			return CompanyDetailsDTO{}, false, err
		}
		result = mapCompanyDetailRow(updatedCompany)
		entry = auditlog.Entry{EntityType: "company", EntityID: existingID, Action: "update", Before: mapCompanyDetailRow(beforeCompany), After: result}
	}

	if s.recorder != nil {
		if err := s.recorder.Record(ctx, tx.queries, entry); err != nil {
			return CompanyDetailsDTO{}, false, err
		}
	}
	if err := tx.commit(ctx); err != nil {
		return CompanyDetailsDTO{}, false, err
	}
	committed = true
	return result, existingID == 0, nil
}

func isNIPUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "check_unique_nip"
}

func buildCreateCompanyParams(req CreateCompanyRequest) (dbsqlc.CreateCompanyParams, error) {
	const maxExpiryNotificationRecipients = 10
	name := strings.TrimSpace(req.Name)
	street := strings.TrimSpace(req.Street)
	city := strings.TrimSpace(req.City)
	zipcode := strings.TrimSpace(req.Zipcode)
	nip := strings.TrimSpace(req.Nip)
	telephone := strings.TrimSpace(req.Telephone)
	expiryEmailPg := pgtype.Text{}
	if req.ExpiryNotificationEmail != nil {
		expiryEmail := strings.TrimSpace(*req.ExpiryNotificationEmail)
		if expiryEmail != "" {
			emails, err := validation.ParseEmailList(expiryEmail)
			if err != nil {
				return dbsqlc.CreateCompanyParams{}, ErrInvalidInput
			}
			if len(emails) > maxExpiryNotificationRecipients {
				return dbsqlc.CreateCompanyParams{}, ErrInvalidInput
			}
			expiryEmails := strings.Join(emails, ",")
			expiryEmailPg = pgtype.Text{String: expiryEmails, Valid: true}
		}
	}

	// Telefon jest opcjonalny - brak zapisujemy jako pusty string, tak jak jest
	// w istniejących danych (kolumna telephoneno pozostaje NOT NULL).
	if name == "" || street == "" || city == "" || zipcode == "" || nip == "" {
		return dbsqlc.CreateCompanyParams{}, ErrInvalidInput
	}
	// NIP zapisujemy jako same cyfry - w tym formacie są wszystkie istniejące
	// wartości, a jednolity zapis pozwala ograniczeniu unikalności wyłapać ten sam
	// NIP wpisany z myślnikami.
	nip = validation.NormalizeNIP(nip)
	if err := validation.ValidateNIP(nip); err != nil {
		return dbsqlc.CreateCompanyParams{}, fmt.Errorf("%w: %w", ErrInvalidNIP, err)
	}

	return dbsqlc.CreateCompanyParams{
		Name:                       name,
		Street:                     street,
		City:                       city,
		Zipcode:                    zipcode,
		Nip:                        nip,
		Email:                      pgutil.OptionalText(req.Email),
		Contactperson:              pgutil.OptionalText(req.ContactPerson),
		Telephoneno:                telephone,
		Note:                       pgutil.OptionalText(req.Note),
		ExpiryNotificationsEnabled: req.ExpiryNotificationsEnabled,
		ExpiryNotificationEmail:    expiryEmailPg,
	}, nil
}

func buildUpdateCompanyParams(companyID int64, req UpdateCompanyDTO) (dbsqlc.UpdateCompanyParams, error) {
	params, err := buildCreateCompanyParams(CreateCompanyRequest(req))
	if err != nil {
		return dbsqlc.UpdateCompanyParams{}, err
	}

	return dbsqlc.UpdateCompanyParams{
		ID:                         companyID,
		Name:                       params.Name,
		Street:                     params.Street,
		City:                       params.City,
		Zipcode:                    params.Zipcode,
		Nip:                        params.Nip,
		Email:                      params.Email,
		Contactperson:              params.Contactperson,
		Telephoneno:                params.Telephoneno,
		Note:                       params.Note,
		ExpiryNotificationsEnabled: params.ExpiryNotificationsEnabled,
		ExpiryNotificationEmail:    params.ExpiryNotificationEmail,
	}, nil
}
