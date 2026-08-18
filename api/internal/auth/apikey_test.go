package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	dbsql "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

// fakeAPIKeyQuerier zastępuje warstwę bazy w testach middleware. Pola funkcyjne
// pozwalają każdemu testowi ustawić własne zachowanie.
type fakeAPIKeyQuerier struct {
	getByHash func(ctx context.Context, tokenHash string) (dbsql.GetActiveAPIKeyByTokenHashRow, error)
	touch     func(ctx context.Context, id int64) error
	touched   []int64
}

func (f *fakeAPIKeyQuerier) GetActiveAPIKeyByTokenHash(ctx context.Context, tokenHash string) (dbsql.GetActiveAPIKeyByTokenHashRow, error) {
	if f.getByHash == nil {
		return dbsql.GetActiveAPIKeyByTokenHashRow{}, errors.New("unexpected lookup")
	}
	return f.getByHash(ctx, tokenHash)
}

func (f *fakeAPIKeyQuerier) TouchAPIKeyLastUsed(ctx context.Context, id int64) error {
	f.touched = append(f.touched, id)
	if f.touch == nil {
		return nil
	}
	return f.touch(ctx, id)
}

func activeKeyRow(scopes []string) dbsql.GetActiveAPIKeyByTokenHashRow {
	return dbsql.GetActiveAPIKeyByTokenHashRow{
		ID:        7,
		Name:      "integracja kadrowa",
		Prefix:    "clk_abcd1234",
		Scopes:    scopes,
		UserID:    42,
		Email:     "integracja@example.com",
		Firstname: "Konto",
		Lastname:  "Serwisowe",
		Role:      0,
	}
}

func capturePrincipal(t *testing.T, captured *Principal) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			t.Error("expected a principal in the request context")
		}
		*captured = principal
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestAPIKeyAuthenticatorAcceptsValidKey(t *testing.T) {
	raw, _, tokenHash, err := NewAPIKeyToken()
	if err != nil {
		t.Fatalf("failed to generate api key: %v", err)
	}

	var lookedUpHash string
	queries := &fakeAPIKeyQuerier{
		getByHash: func(_ context.Context, hash string) (dbsql.GetActiveAPIKeyByTokenHashRow, error) {
			lookedUpHash = hash
			return activeKeyRow([]string{ScopeStudentsRead}), nil
		},
	}

	var principal Principal
	handler := Authenticate(APIKeyAuthenticator(queries))(capturePrincipal(t, &principal))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if lookedUpHash != tokenHash {
		t.Fatal("middleware must look the key up by its hash")
	}
	if lookedUpHash == raw {
		t.Fatal("raw key must never be used as the lookup value")
	}
	if principal.Method != MethodAPIKey {
		t.Fatalf("expected api_key method, got %q", principal.Method)
	}
	if principal.User.ID != 42 || principal.User.Email != "integracja@example.com" {
		t.Fatalf("expected the service account in context, got %+v", principal.User)
	}
	if principal.APIKeyID != 7 || principal.APIKeyName != "integracja kadrowa" {
		t.Fatalf("expected key identity in principal, got id=%d name=%q", principal.APIKeyID, principal.APIKeyName)
	}
	if len(queries.touched) != 1 || queries.touched[0] != 7 {
		t.Fatalf("expected last usage to be recorded once, got %v", queries.touched)
	}
}

func TestAPIKeyAuthenticatorPutsUserInContextForHandlers(t *testing.T) {
	// Handlery czytają wyłącznie UserFromContext - to jest gwarancja, że nie
	// muszą wiedzieć nic o kluczach API.
	raw, _, _, err := NewAPIKeyToken()
	if err != nil {
		t.Fatalf("failed to generate api key: %v", err)
	}

	queries := &fakeAPIKeyQuerier{
		getByHash: func(context.Context, string) (dbsql.GetActiveAPIKeyByTokenHashRow, error) {
			return activeKeyRow([]string{ScopeStudentsRead}), nil
		},
	}

	var seen dbsql.User
	handler := Authenticate(APIKeyAuthenticator(queries))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			t.Error("expected a user in the request context")
		}
		seen = user
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if seen.ID != 42 {
		t.Fatalf("expected service account id 42, got %d", seen.ID)
	}
}

func TestAPIKeyAuthenticatorRejectsUnknownKey(t *testing.T) {
	queries := &fakeAPIKeyQuerier{
		getByHash: func(context.Context, string) (dbsql.GetActiveAPIKeyByTokenHashRow, error) {
			return dbsql.GetActiveAPIKeyByTokenHashRow{}, pgx.ErrNoRows
		},
	}

	handler := Authenticate(APIKeyAuthenticator(queries))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run for an unknown key")
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	req.Header.Set("Authorization", "Bearer clk_nieistniejacy")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestAPIKeyAuthenticatorRejectsMalformedHeader(t *testing.T) {
	for _, header := range []string{"Bearer", "Bearer ", "Basic abc"} {
		t.Run(header, func(t *testing.T) {
			queries := &fakeAPIKeyQuerier{
				getByHash: func(context.Context, string) (dbsql.GetActiveAPIKeyByTokenHashRow, error) {
					t.Error("malformed header must not reach the database")
					return dbsql.GetActiveAPIKeyByTokenHashRow{}, nil
				},
			}

			handler := Authenticate(APIKeyAuthenticator(queries))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				t.Error("handler must not run for a malformed header")
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
			req.Header.Set("Authorization", header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
		})
	}
}

func TestAPIKeyAuthenticatorIgnoresTouchFailure(t *testing.T) {
	// Znacznik ostatniego użycia jest wygodą operacyjną - jego błąd nie może
	// odciąć działającej integracji.
	raw, _, _, err := NewAPIKeyToken()
	if err != nil {
		t.Fatalf("failed to generate api key: %v", err)
	}

	queries := &fakeAPIKeyQuerier{
		getByHash: func(context.Context, string) (dbsql.GetActiveAPIKeyByTokenHashRow, error) {
			return activeKeyRow([]string{ScopeStudentsRead}), nil
		},
		touch: func(context.Context, int64) error {
			return errors.New("database is busy")
		},
	}

	handler := Authenticate(APIKeyAuthenticator(queries))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
}

func TestAuthenticateWithoutAnyCredentialsReturnsUnauthorized(t *testing.T) {
	handler := Authenticate(
		SessionAuthenticator(dbsql.New(fakeDB{}), testConfig()),
		APIKeyAuthenticator(&fakeAPIKeyQuerier{}),
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run without credentials")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestAuthenticatePrefersSessionCookieOverAPIKey(t *testing.T) {
	// Przeglądarka wysyła ciasteczko przy każdym żądaniu; gdyby jednocześnie
	// trafił się nagłówek Authorization, sesja ma wygrać - inaczej zalogowany
	// człowiek działałby nagle z ograniczeniami klucza.
	sessionQueries := dbsql.New(fakeDB{
		queryRow: func(_ context.Context, sql string, _ ...interface{}) pgx.Row {
			if strings.Contains(sql, "api_sessions") {
				return fakeRow{scan: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "hashed"
					*(dest[1].(*int64)) = 1
					*(dest[2].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}
					*(dest[3].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now(), Valid: true}
					return nil
				}}
			}
			return fakeRow{scan: func(dest ...interface{}) error {
				*(dest[0].(*int64)) = 1
				*(dest[1].(*string)) = "czlowiek@example.com"
				*(dest[2].(*[]byte)) = []byte("hash")
				*(dest[3].(*string)) = "Jan"
				*(dest[4].(*string)) = "Nowak"
				*(dest[5].(*int32)) = 0
				return nil
			}}
		},
	})

	apiKeyQueries := &fakeAPIKeyQuerier{
		getByHash: func(context.Context, string) (dbsql.GetActiveAPIKeyByTokenHashRow, error) {
			t.Error("api key must not be consulted when a valid session cookie is present")
			return dbsql.GetActiveAPIKeyByTokenHashRow{}, nil
		},
	}

	var principal Principal
	handler := Authenticate(
		SessionAuthenticator(sessionQueries, testConfig()),
		APIKeyAuthenticator(apiKeyQueries),
	)(capturePrincipal(t, &principal))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "raw-token"})
	req.Header.Set("Authorization", "Bearer clk_cokolwiek")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if principal.Method != MethodSession {
		t.Fatalf("expected session method, got %q", principal.Method)
	}
	if len(principal.Scopes) != 0 {
		t.Fatalf("session principal must carry no scopes, got %v", principal.Scopes)
	}
}

func TestNewAPIKeyTokenIsPrefixedAndHashed(t *testing.T) {
	raw, prefix, tokenHash, err := NewAPIKeyToken()
	if err != nil {
		t.Fatalf("failed to generate api key: %v", err)
	}

	if !strings.HasPrefix(raw, APIKeyTokenPrefix) {
		t.Fatalf("expected key to start with %q, got %q", APIKeyTokenPrefix, raw)
	}
	if !strings.HasPrefix(raw, prefix) || len(prefix) >= len(raw) {
		t.Fatalf("prefix %q must be a proper prefix of the key", prefix)
	}
	if tokenHash == raw || strings.Contains(tokenHash, raw) {
		t.Fatal("stored hash must not contain the raw key")
	}
	if tokenHash != HashToken(raw) {
		t.Fatal("stored hash must match HashToken of the raw key")
	}

	other, _, _, err := NewAPIKeyToken()
	if err != nil {
		t.Fatalf("failed to generate second api key: %v", err)
	}
	if other == raw {
		t.Fatal("two generated keys must differ")
	}
}

func TestHasScope(t *testing.T) {
	tests := []struct {
		name     string
		granted  []string
		required string
		want     bool
	}{
		{name: "dokładne dopasowanie", granted: []string{ScopeStudentsRead}, required: ScopeStudentsRead, want: true},
		{name: "zapis implikuje odczyt", granted: []string{ScopeStudentsWrite}, required: ScopeStudentsRead, want: true},
		{name: "odczyt nie implikuje zapisu", granted: []string{ScopeStudentsRead}, required: ScopeStudentsWrite, want: false},
		{name: "inny zasób nie przechodzi", granted: []string{ScopeCoursesWrite}, required: ScopeStudentsRead, want: false},
		{name: "brak zakresów", granted: nil, required: ScopeStudentsRead, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasScope(tc.granted, tc.required); got != tc.want {
				t.Fatalf("HasScope(%v, %q) = %v, want %v", tc.granted, tc.required, got, tc.want)
			}
		})
	}
}

func TestRequireScopeBlocksKeyWithoutScope(t *testing.T) {
	handler := RequireScope(ScopeStudentsWrite)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run without the required scope")
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/students", nil)
	req = req.WithContext(ContextWithPrincipal(req.Context(), Principal{
		Method: MethodAPIKey,
		Scopes: []string{ScopeStudentsRead},
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusForbidden, response.CodeForbidden)
}

func TestRequireScopeAllowsKeyWithScope(t *testing.T) {
	handler := RequireScope(ScopeStudentsRead)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	req = req.WithContext(ContextWithPrincipal(req.Context(), Principal{
		Method: MethodAPIKey,
		Scopes: []string{ScopeStudentsWrite},
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
}

func TestRequireScopeAllowsSession(t *testing.T) {
	// Zakresy ograniczają wyłącznie klucze API; o dostępie zalogowanego
	// użytkownika decyduje jego rola.
	handler := RequireScope(ScopeUsersWrite)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", nil)
	req = req.WithContext(ContextWithPrincipal(req.Context(), Principal{Method: MethodSession}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
}

func TestRequireScopeWithoutPrincipalReturnsUnauthorized(t *testing.T) {
	handler := RequireScope(ScopeStudentsRead)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run without an authenticated principal")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/students", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestRequireSessionRejectsAPIKey(t *testing.T) {
	handler := RequireSession()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run for an api key")
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/password", nil)
	req = req.WithContext(ContextWithPrincipal(req.Context(), Principal{
		Method: MethodAPIKey,
		Scopes: []string{ScopeUsersWrite},
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusForbidden, response.CodeForbidden)
}

func TestRequireSessionAllowsSession(t *testing.T) {
	handler := RequireSession()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/account/password", nil)
	req = req.WithContext(ContextWithPrincipal(req.Context(), Principal{Method: MethodSession}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
}

func TestIsAssignableScopeRejectsUnknownValues(t *testing.T) {
	if !IsAssignableScope(ScopeCertificatesWrite) {
		t.Fatal("expected a catalog scope to be assignable")
	}
	for _, scope := range []string{"", "*", "certificates", "certificates:delete", "CERTIFICATES:READ"} {
		if IsAssignableScope(scope) {
			t.Fatalf("scope %q must not be assignable", scope)
		}
	}
}
