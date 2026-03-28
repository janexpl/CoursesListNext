// Package auth ..
package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/janexpl/CoursesListNext/api/internal/config"
	dbsqlc "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type contextKey string

const (
	userContextKey contextKey = "auth_user"
	RoleAdmin      int32      = 1
)

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

func RequireAuth(queries *dbsqlc.Queries, config *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(config.SessionCookieName)
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "missing session token")
				return
			}
			token := cookie.Value
			session, err := queries.GetSessionByToken(r.Context(), token)
			if err != nil || session.ExpiresAt.Time.Before(time.Now()) {
				clearSessionCookie(w, config)
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "invalid or expired token")
				return
			}
			user, err := queries.GetUserByID(r.Context(), session.UserID)
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "user not found")
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
