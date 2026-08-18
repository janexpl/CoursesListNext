// Package auth ..
package auth

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/janexpl/CoursesListNext/api/internal/config"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type contextKey string

const (
	userContextKey      contextKey = "auth_user"
	principalContextKey contextKey = "auth_principal"
	RoleAdmin           int32      = 1
)

// Authenticator próbuje uwierzytelnić żądanie jedną metodą.
//
// Drugi zwracany parametr mówi, czy poświadczenia tej metody w ogóle wystąpiły
// w żądaniu. false oznacza "nie moja sprawa" i pozwala Authenticate przejść do
// kolejnej metody; true z błędem kończy żądanie odpowiedzią 401 - poświadczenia
// były, ale nie są poprawne, więc próbowanie dalszych metod tylko zamazałoby
// przyczynę odmowy.
//
// ResponseWriter jest przekazywany, bo uwierzytelnianie sesją musi wyczyścić
// nieważne ciasteczko.
type Authenticator func(http.ResponseWriter, *http.Request) (Principal, bool, error)

// Authenticate składa metody uwierzytelniania w jedno middleware. Metody są
// próbowane w podanej kolejności, a pierwsza pasująca decyduje o wyniku.
func Authenticate(authenticators ...Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, authenticate := range authenticators {
				principal, matched, err := authenticate(w, r)
				if !matched {
					continue
				}
				if err != nil {
					response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
					return
				}
				next.ServeHTTP(w, r.WithContext(ContextWithPrincipal(r.Context(), principal)))
				return
			}
			response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "missing credentials")
		})
	}
}

// RequireAuth uwierzytelnia wyłącznie ciasteczkiem sesji.
func RequireAuth(queries *dbsqlc.Queries, config *config.Config) func(http.Handler) http.Handler {
	return Authenticate(SessionAuthenticator(queries, config))
}

// SessionAuthenticator obsługuje logowanie z przeglądarki: token z ciasteczka
// HTTP-only wyszukany w api_sessions.
func SessionAuthenticator(queries sessionQuerier, config *config.Config) Authenticator {
	return func(w http.ResponseWriter, r *http.Request) (Principal, bool, error) {
		cookie, err := r.Cookie(config.SessionCookieName)
		if err != nil {
			return Principal{}, false, nil
		}

		session, err := queries.GetSessionByToken(r.Context(), hashToken(cookie.Value))
		if err != nil || session.ExpiresAt.Time.Before(time.Now()) {
			clearSessionCookie(w, config)
			return Principal{}, true, errInvalidSession
		}
		user, err := queries.GetUserByID(r.Context(), session.UserID)
		if err != nil {
			clearSessionCookie(w, config)
			return Principal{}, true, errInvalidSession
		}

		return Principal{User: user, Method: MethodSession}, true, nil
	}
}

// APIKeyAuthenticator obsługuje integracje serwer-serwer: nagłówek
// Authorization: Bearer <klucz> sprawdzany w api_keys.
func APIKeyAuthenticator(queries apiKeyQuerier) Authenticator {
	return func(_ http.ResponseWriter, r *http.Request) (Principal, bool, error) {
		raw, present := bearerToken(r)
		if !present {
			return Principal{}, false, nil
		}
		if raw == "" {
			return Principal{}, true, errInvalidAPIKey
		}

		row, err := queries.GetActiveAPIKeyByTokenHash(r.Context(), hashToken(raw))
		if err != nil {
			return Principal{}, true, errInvalidAPIKey
		}

		// Znacznik ostatniego użycia jest wygodą operacyjną, nie częścią
		// uwierzytelnienia - jego błąd nie może blokować żądania.
		if err := queries.TouchAPIKeyLastUsed(r.Context(), row.ID); err != nil {
			log.Printf("unable to update api key last usage: %v", err)
		}

		return Principal{
			User: dbsqlc.User{
				ID:        row.UserID,
				Email:     row.Email,
				Password:  row.Password,
				Firstname: row.Firstname,
				Lastname:  row.Lastname,
				Role:      row.Role,
			},
			Method:     MethodAPIKey,
			APIKeyID:   row.ID,
			APIKeyName: row.Name,
			Scopes:     row.Scopes,
		}, true, nil
	}
}

// RequireScope przepuszcza żądanie, jeśli klucz API ma wymagany zakres.
// Sesje przechodzą bez sprawdzania - zakresy ograniczają wyłącznie klucze,
// a uprawnienia zalogowanego użytkownika wynikają z jego roli.
func RequireScope(required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthenticated request")
				return
			}
			if principal.Method == MethodAPIKey && !HasScope(principal.Scopes, required) {
				response.WriteError(w, http.StatusForbidden, response.CodeForbidden, "api key is missing the "+required+" scope")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSession blokuje trasy, które mają sens wyłącznie dla zalogowanego
// człowieka (zarządzanie własnym kontem, wylogowanie).
func RequireSession() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := PrincipalFromContext(r.Context())
			if !ok {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthenticated request")
				return
			}
			if principal.Method != MethodSession {
				response.WriteError(w, http.StatusForbidden, response.CodeForbidden, "this endpoint requires an interactive session")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := userFromContext(r.Context())
			if !ok {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "user not found in context")
				return
			}
			if user.Role != RoleAdmin {
				response.WriteError(w, http.StatusForbidden, response.CodeForbidden, "admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type sessionQuerier interface {
	GetSessionByToken(ctx context.Context, token string) (dbsqlc.ApiSession, error)
	GetUserByID(ctx context.Context, id int64) (dbsqlc.User, error)
}

type apiKeyQuerier interface {
	GetActiveAPIKeyByTokenHash(ctx context.Context, tokenHash string) (dbsqlc.GetActiveAPIKeyByTokenHashRow, error)
	TouchAPIKeyLastUsed(ctx context.Context, id int64) error
}
