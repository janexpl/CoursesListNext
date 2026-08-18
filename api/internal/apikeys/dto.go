// Package apikeys - zarządzanie kluczami API dla integracji serwer-serwer.
package apikeys

import (
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
)

// APIKeyDTO nigdy nie zawiera surowego klucza - ten wraca wyłącznie raz,
// w CreateAPIKeyResponse.
type APIKeyDTO struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	UserID     int64    `json:"userId"`
	UserEmail  string   `json:"userEmail,omitempty"`
	UserName   string   `json:"userName,omitempty"`
	Scopes     []string `json:"scopes"`
	ExpiresAt  *string  `json:"expiresAt"`
	LastUsedAt *string  `json:"lastUsedAt"`
	RevokedAt  *string  `json:"revokedAt"`
	CreatedAt  *string  `json:"createdAt"`
}

type CreateAPIKeyRequest struct {
	Name string `json:"name"`
	// UserID wskazuje konto serwisowe, w którego imieniu działa klucz. Rola
	// tego konta nadal decyduje o dostępie do tras administracyjnych.
	UserID int64    `json:"userId"`
	Scopes []string `json:"scopes"`
	// ExpiresAt w formacie YYYY-MM-DD; puste oznacza klucz bezterminowy.
	ExpiresAt string `json:"expiresAt"`
}

type ListAPIKeysResponse struct {
	Data []APIKeyDTO `json:"data"`
}

type CreateAPIKeyResponse struct {
	Data APIKeyDTO `json:"data"`
	// Token jest jedyną okazją do skopiowania klucza - w bazie leży tylko
	// jego skrót.
	Token string `json:"token"`
}

type ScopesResponse struct {
	Data []string `json:"data"`
}

func mapListRow(row sqlc.ListAPIKeysRow) APIKeyDTO {
	return APIKeyDTO{
		ID:         row.ID,
		Name:       row.Name,
		Prefix:     row.Prefix,
		UserID:     row.UserID,
		UserEmail:  row.UserEmail,
		UserName:   row.UserFirstname + " " + row.UserLastname,
		Scopes:     scopesOrEmpty(row.Scopes),
		ExpiresAt:  pgutil.NullableTimestampz(row.ExpiresAt),
		LastUsedAt: pgutil.NullableTimestampz(row.LastUsedAt),
		RevokedAt:  pgutil.NullableTimestampz(row.RevokedAt),
		CreatedAt:  pgutil.NullableTimestampz(row.CreatedAt),
	}
}

func mapCreateRow(row sqlc.CreateAPIKeyRow) APIKeyDTO {
	return APIKeyDTO{
		ID:         row.ID,
		Name:       row.Name,
		Prefix:     row.Prefix,
		UserID:     row.UserID,
		Scopes:     scopesOrEmpty(row.Scopes),
		ExpiresAt:  pgutil.NullableTimestampz(row.ExpiresAt),
		LastUsedAt: pgutil.NullableTimestampz(row.LastUsedAt),
		RevokedAt:  pgutil.NullableTimestampz(row.RevokedAt),
		CreatedAt:  pgutil.NullableTimestampz(row.CreatedAt),
	}
}

// scopesOrEmpty pilnuje, żeby JSON miał [] zamiast null - frontend iteruje po
// tej liście bez sprawdzania.
func scopesOrEmpty(scopes []string) []string {
	if scopes == nil {
		return []string{}
	}
	return scopes
}
