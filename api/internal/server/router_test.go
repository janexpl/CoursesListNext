package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/janexpl/CoursesListNext/api/internal/config"
	dbsql "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

// testRouter składa router bez bazy. Rejestracja tras w chi panikuje dopiero
// w czasie działania (kolizja ścieżek, zły wzorzec), więc samo zbudowanie
// routera jest tu istotnym asertem.
func testRouter(t *testing.T) http.Handler {
	t.Helper()
	return NewRouter(Dependencies{
		Queries: dbsql.New(nil),
		Config: &config.Config{
			SessionCookieName:  "session_token",
			CORSAllowedOrigins: []string{"http://localhost:3000"},
			LoginRateLimit:     5,
		},
	})
}

func TestRouterBuildsWithoutRouteCollisions(t *testing.T) {
	if testRouter(t) == nil {
		t.Fatal("expected a router")
	}
}

func TestProtectedRoutesRejectAnonymousRequests(t *testing.T) {
	router := testRouter(t)

	// Żądanie bez ciasteczka i bez nagłówka Authorization musi odpaść na
	// uwierzytelnianiu, zanim dotknie bazy - deps.Queries jest tu nil.
	for _, target := range []string{
		"/api/v1/students",
		"/api/v1/certificates",
		"/api/v1/journals",
		"/api/v1/admin/users",
		"/api/v1/admin/api-keys",
	} {
		t.Run(target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401 for %s, got %d", target, rec.Code)
			}
		})
	}
}

func TestHealthzStaysPublic(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestCORSAllowsAuthorizationHeader(t *testing.T) {
	// Bez tego przeglądarkowy klient z innej domeny nie mógłby wysłać klucza API.
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/students", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, req)

	if allowed := rec.Header().Get("Access-Control-Allow-Headers"); allowed == "" {
		t.Fatalf("expected Authorization to be an allowed header, got %q", allowed)
	}
}

func TestInternalNotificationsRouteStillRequiresBearerToken(t *testing.T) {
	// Trasa dla crona nie przeszła na klucze API - musi dalej odpowiadać 401
	// na żądanie bez nagłówka, a nie wpuszczać sesji.
	router := NewRouter(Dependencies{
		Queries: dbsql.New(nil),
		Config: &config.Config{
			SessionCookieName:     "session_token",
			CORSAllowedOrigins:    []string{"http://localhost:3000"},
			LoginRateLimit:        5,
			NotificationsAPIToken: "sekret",
		},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/internal/notifications/expiring-certificates", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	var body response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != response.CodeUnauthorized {
		t.Fatalf("expected error code %q, got %q", response.CodeUnauthorized, body.Error.Code)
	}
}
