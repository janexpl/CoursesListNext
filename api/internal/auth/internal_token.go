package auth

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/janexpl/CoursesListNext/api/internal/response"
)

var (
	errInvalidSession = errors.New("invalid or expired token")
	errInvalidAPIKey  = errors.New("invalid or expired api key")
)

// bearerToken wyciąga token z nagłówka Authorization. Drugi zwracany parametr
// mówi, czy nagłówek w ogóle wystąpił - pusty token przy obecnym nagłówku to
// błędne poświadczenie, a nie brak poświadczeń.
func bearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		return "", true
	}
	return strings.TrimSpace(token), true
}

func RequireBearerToken(expectedToken string) func(http.Handler) http.Handler {
	expectedToken = strings.TrimSpace(expectedToken)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectedToken == "" {
				response.WriteError(w, http.StatusServiceUnavailable, response.CodeInternalError, "internal token is not found")
				return
			}

			token, present := bearerToken(r)
			if !present {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "missing bearer token")
				return
			}
			if token == "" {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "invalid bearer token")
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
				response.WriteError(w, http.StatusUnauthorized, response.CodeUnauthorized, "invalid bearer token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
