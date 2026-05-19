package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/janexpl/CoursesListNext/api/internal/response"
)

func RequireBearerToken(expectedToken string) func(http.Handler) http.Handler {
	expectedToken = strings.TrimSpace(expectedToken)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectedToken == "" {
				response.WriteError(w, http.StatusServiceUnavailable, response.CodeInternalError, "internal token is not found")
				return
			}

			header := strings.TrimSpace(r.Header.Get("Authorization"))
			if header == "" {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "missing bearer token")
				return
			}

			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || strings.TrimSpace(token) == "" {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "invalid bearer token")
				return
			}

			token = strings.TrimSpace(token)
			if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "invalid bearer token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
