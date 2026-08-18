package apikeys

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/janexpl/CoursesListNext/api/internal/auditlog"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrUnknownScope   = errors.New("unknown scope")
	ErrNoScopes       = errors.New("at least one scope is required")
	ErrExpiryInPast   = errors.New("expiry date must be in the future")
	ErrUserNotFound   = errors.New("service account not found")
	ErrAlreadyRevoked = errors.New("api key is already revoked")
)

type serviceQuerier interface {
	GetUserByID(ctx context.Context, id int64) (sqlc.User, error)
	GetAPIKeyByID(ctx context.Context, id int64) (sqlc.GetAPIKeyByIDRow, error)
}

type txScope struct {
	queries  *sqlc.Queries
	commit   func(context.Context) error
	rollback func(context.Context) error
}

type Service struct {
	queries  serviceQuerier
	recorder *auditlog.Recorder
	beginTx  func(context.Context) (txScope, error)
}

func NewService(pool *pgxpool.Pool, queries *sqlc.Queries, recorder *auditlog.Recorder) *Service {
	return &Service{
		queries:  queries,
		recorder: recorder,
		beginTx: func(ctx context.Context) (txScope, error) {
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

// Create wystawia nowy klucz i zwraca go w postaci surowej tylko w tym jednym
// miejscu. Zapis klucza i wpis do audit logu idą w jednej transakcji, żeby nie
// dało się wystawić klucza bez śladu, kto go utworzył.
func (s *Service) Create(ctx context.Context, req CreateAPIKeyRequest) (APIKeyDTO, string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || req.UserID <= 0 {
		return APIKeyDTO{}, "", ErrInvalidInput
	}

	scopes, err := normalizeScopes(req.Scopes)
	if err != nil {
		return APIKeyDTO{}, "", err
	}

	expiresAt, err := parseExpiry(req.ExpiresAt)
	if err != nil {
		return APIKeyDTO{}, "", err
	}

	// Klucz musi wskazywać istniejące konto, inaczej zapis padłby dopiero na
	// kluczu obcym z komunikatem nieczytelnym dla wołającego.
	serviceAccount, err := s.queries.GetUserByID(ctx, req.UserID)
	if err != nil {
		return APIKeyDTO{}, "", ErrUserNotFound
	}

	raw, prefix, tokenHash, err := auth.NewAPIKeyToken()
	if err != nil {
		return APIKeyDTO{}, "", err
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return APIKeyDTO{}, "", err
	}
	committed := false
	defer func() {
		if !committed {
			if err := tx.rollback(ctx); err != nil {
				log.Printf("unable to rollback changes: %v", err)
			}
		}
	}()

	created, err := tx.queries.CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		Name:      name,
		Prefix:    prefix,
		TokenHash: tokenHash,
		UserID:    req.UserID,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return APIKeyDTO{}, "", err
	}

	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "api_key",
		EntityID:   created.ID,
		Action:     "create",
		Before:     nil,
		After: map[string]any{
			"name":             created.Name,
			"prefix":           created.Prefix,
			"scopes":           created.Scopes,
			"serviceAccountId": created.UserID,
			"expiresAt":        formatTimestamp(created.ExpiresAt),
		},
		Metadata: map[string]any{
			"serviceAccountEmail": serviceAccount.Email,
			"serviceAccountRole":  serviceAccount.Role,
		},
	}); err != nil {
		return APIKeyDTO{}, "", err
	}

	if err := tx.commit(ctx); err != nil {
		return APIKeyDTO{}, "", err
	}
	committed = true

	dto := mapCreateRow(created)
	dto.UserEmail = serviceAccount.Email
	dto.UserName = serviceAccount.Firstname + " " + serviceAccount.Lastname
	return dto, raw, nil
}

// Revoke unieważnia klucz nieodwracalnie. Klucze nie są kasowane, żeby wpisy
// w audit logu wskazywały na istniejący rekord.
func (s *Service) Revoke(ctx context.Context, id int64) error {
	existing, err := s.queries.GetAPIKeyByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.RevokedAt.Valid {
		return ErrAlreadyRevoked
	}

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

	affected, err := tx.queries.RevokeAPIKey(ctx, id)
	if err != nil {
		return err
	}
	if affected == 0 {
		// Ktoś unieważnił ten klucz między odczytem a zapisem.
		return ErrAlreadyRevoked
	}

	if err := s.recorder.Record(ctx, tx.queries, auditlog.Entry{
		EntityType: "api_key",
		EntityID:   id,
		Action:     "delete",
		Before: map[string]any{
			"name":             existing.Name,
			"prefix":           existing.Prefix,
			"scopes":           existing.Scopes,
			"serviceAccountId": existing.UserID,
		},
		After:    nil,
		Metadata: map[string]any{"mode": "revoke"},
	}); err != nil {
		return err
	}

	if err := tx.commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

// normalizeScopes przycina, deduplikuje i weryfikuje zakresy. Pusta lista jest
// odrzucana, bo klucz bez zakresów nie miałby dostępu do niczego, a wyglądałby
// na działający.
func normalizeScopes(scopes []string) ([]string, error) {
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if !auth.IsAssignableScope(scope) {
			return nil, ErrUnknownScope
		}
		if !slices.Contains(normalized, scope) {
			normalized = append(normalized, scope)
		}
	}
	if len(normalized) == 0 {
		return nil, ErrNoScopes
	}
	slices.Sort(normalized)
	return normalized, nil
}

// parseExpiry zamienia datę YYYY-MM-DD na koniec tego dnia w czasie lokalnym -
// klucz ważny "do 31 grudnia" ma działać przez cały 31 grudnia.
func parseExpiry(value string) (pgtype.Timestamptz, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return pgtype.Timestamptz{}, nil
	}
	parsed, err := time.ParseInLocation(response.DateFormat, value, time.Local)
	if err != nil {
		return pgtype.Timestamptz{}, ErrInvalidInput
	}
	endOfDay := parsed.Add(24*time.Hour - time.Second)
	if !endOfDay.After(time.Now()) {
		return pgtype.Timestamptz{}, ErrExpiryInPast
	}
	return pgtype.Timestamptz{Time: endOfDay, Valid: true}, nil
}

func formatTimestamp(value pgtype.Timestamptz) any {
	if !value.Valid {
		return nil
	}
	return value.Time.Format(response.TimestampzFormat)
}
