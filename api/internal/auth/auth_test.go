package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/config"
	dbsql "github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
	"golang.org/x/crypto/bcrypt"
)

type fakeDB struct {
	exec     func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	queryRow func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (f fakeDB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if f.exec == nil {
		return pgconn.CommandTag{}, errors.New("unexpected exec call")
	}
	return f.exec(ctx, sql, args...)
}

func (f fakeDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query call")
}

func (f fakeDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if f.queryRow == nil {
		return fakeRow{err: errors.New("unexpected query row call")}
	}
	return f.queryRow(ctx, sql, args...)
}

type fakeRow struct {
	scan func(dest ...interface{}) error
	err  error
}

func (r fakeRow) Scan(dest ...interface{}) error {
	if r.scan != nil {
		return r.scan(dest...)
	}
	return r.err
}

func testConfig() *config.Config {
	return &config.Config{
		SessionTTL:          24 * time.Hour,
		SessionCookieName:   "session_token",
		SessionCookieSecure: false,
	}
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
	t.Helper()

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if responseBody.Error.Code != expectedCode {
		t.Fatalf("expected error code %q, got %q", expectedCode, responseBody.Error.Code)
	}
}

func TestLoginBadJSONReturnsBadRequest(t *testing.T) {
	handler := NewHandler(dbsql.New(fakeDB{}), testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestLoginMissingCredentialsReturnsBadRequest(t *testing.T) {
	handler := NewHandler(dbsql.New(fakeDB{}), testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"","password":""}`))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestLoginInvalidCredentialsReturnsUnauthorized(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	queries := dbsql.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...interface{}) pgx.Row {
			return fakeRow{
				scan: func(dest ...interface{}) error {
					*(dest[0].(*int64)) = 1
					*(dest[1].(*string)) = "user@example.com"
					*(dest[2].(*[]byte)) = hash
					*(dest[3].(*string)) = "Jan"
					*(dest[4].(*string)) = "Nowak"
					*(dest[5].(*int32)) = 1
					return nil
				},
			}
		},
	})

	handler := NewHandler(queries, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"wrong-password"}`))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeInvalidCredentials)
}

func TestLoginSetsSessionCookieOnSuccess(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	callCount := 0
	queries := dbsql.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...interface{}) pgx.Row {
			callCount++

			switch callCount {
			case 1:
				return fakeRow{
					scan: func(dest ...interface{}) error {
						*(dest[0].(*int64)) = 1
						*(dest[1].(*string)) = "user@example.com"
						*(dest[2].(*[]byte)) = hash
						*(dest[3].(*string)) = "Jan"
						*(dest[4].(*string)) = "Nowak"
						*(dest[5].(*int32)) = 1
						return nil
					},
				}
			case 2:
				return fakeRow{
					scan: func(dest ...interface{}) error {
						now := time.Now().Add(24 * time.Hour)
						*(dest[0].(*string)) = "generated-token"
						*(dest[1].(*int64)) = 1
						*(dest[2].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: now, Valid: true}
						*(dest[3].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now(), Valid: true}
						return nil
					},
				}
			default:
				return fakeRow{err: errors.New("unexpected query row call")}
			}
		},
	})

	handler := NewHandler(queries, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"user@example.com","password":"correct-password"}`))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "session_token=") {
		t.Fatalf("expected session cookie, got %q", setCookie)
	}
}

func TestRequireAuthMissingCookieReturnsUnauthorized(t *testing.T) {
	middleware := RequireAuth(dbsql.New(fakeDB{}), testConfig())
	next := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	next.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestLoginUnknownFieldReturnsBadRequest(t *testing.T) {
	handler := NewHandler(dbsql.New(fakeDB{}), testConfig())

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/login",
		strings.NewReader(`{"email":"user@example.com","password":"secret","extra":"oops"}`),
	)
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestLoginCreateSessionReturnsInternalServerError(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	callCount := 0
	queries := dbsql.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...interface{}) pgx.Row {
			callCount++

			switch callCount {
			case 1:
				return fakeRow{
					scan: func(dest ...interface{}) error {
						*(dest[0].(*int64)) = 1
						*(dest[1].(*string)) = "user@example.com"
						*(dest[2].(*[]byte)) = hash
						*(dest[3].(*string)) = "Jan"
						*(dest[4].(*string)) = "Nowak"
						*(dest[5].(*int32)) = 1
						return nil
					},
				}
			case 2:
				return fakeRow{err: errors.New("db error")}
			default:
				return fakeRow{err: errors.New("unexpected query row call")}
			}
		},
	})

	handler := NewHandler(queries, testConfig())

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/login",
		strings.NewReader(`{"email":"user@example.com","password":"correct-password"}`),
	)
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestRequireAuthExpiredSessionClearsCookie(t *testing.T) {
	queries := dbsql.New(fakeDB{
		queryRow: func(_ context.Context, _ string, _ ...interface{}) pgx.Row {
			return fakeRow{
				scan: func(dest ...interface{}) error {
					*(dest[0].(*string)) = "expired-token"
					*(dest[1].(*int64)) = 1
					*(dest[2].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true}
					*(dest[3].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true}
					return nil
				},
			}
		},
	})

	middleware := RequireAuth(queries, testConfig())
	next := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "expired-token"})
	rec := httptest.NewRecorder()

	next.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)

	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "session_token=") {
		t.Fatalf("expected clearing session cookie, got %q", setCookie)
	}
}

func TestLogoutMissingCookieReturnsUnauthorized(t *testing.T) {
	handler := NewHandler(dbsql.New(fakeDB{}), testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestLogoutDeleteSessionReturnsInternalServerError(t *testing.T) {
	queries := dbsql.New(fakeDB{
		exec: func(_ context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db error")
		},
	})

	handler := NewHandler(queries, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "logout-token"})
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestLogoutClearsCookieOnSuccess(t *testing.T) {
	var deletedToken string

	queries := dbsql.New(fakeDB{
		exec: func(_ context.Context, _ string, args ...interface{}) (pgconn.CommandTag, error) {
			if len(args) != 1 {
				t.Fatalf("expected 1 arg, got %d", len(args))
			}
			token, ok := args[0].(string)
			if !ok {
				t.Fatalf("expected token arg to be string, got %T", args[0])
			}
			deletedToken = token
			return pgconn.CommandTag{}, nil
		},
	})

	handler := NewHandler(queries, testConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_token", Value: "logout-token"})
	rec := httptest.NewRecorder()

	handler.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if deletedToken != "logout-token" {
		t.Fatalf("expected deleted token %q, got %q", "logout-token", deletedToken)
	}

	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "session_token=") {
		t.Fatalf("expected cleared session cookie, got %q", setCookie)
	}
	if !strings.Contains(setCookie, "Max-Age=0") && !strings.Contains(setCookie, "Max-Age=-1") {
		t.Fatalf("expected cookie to be cleared, got %q", setCookie)
	}
}

func TestMeWithoutUserReturnsUnauthorized(t *testing.T) {
	handler := NewHandler(dbsql.New(fakeDB{}), testConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	handler.Me(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestMeReturnsCurrentUser(t *testing.T) {
	currentUser := dbsql.User{
		ID:        9,
		Email:     "user@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
		Role:      1,
	}

	handler := NewHandler(dbsql.New(fakeDB{}), testConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, currentUser))
	rec := httptest.NewRecorder()

	handler.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"email":"user@example.com"`) {
		t.Fatalf("expected response body to contain user email, got %q", rec.Body.String())
	}
}

func TestUserFromContextReturnsUser(t *testing.T) {
	expected := dbsql.User{
		ID:        7,
		Email:     "user@example.com",
		Firstname: "Jan",
		Lastname:  "Nowak",
		Role:      1,
	}

	ctx := context.WithValue(context.Background(), userContextKey, expected)
	user, ok := userFromContext(ctx)
	if !ok {
		t.Fatal("expected user in context")
	}
	if user.ID != expected.ID || user.Email != expected.Email || user.Firstname != expected.Firstname || user.Lastname != expected.Lastname || user.Role != expected.Role {
		t.Fatalf("expected %+v, got %+v", expected, user)
	}
}

func TestRequireAdminWithoutUserInContextReturnsUnauthorized(t *testing.T) {
	middleware := RequireAdmin()
	next := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1", nil)
	rec := httptest.NewRecorder()

	next.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusUnauthorized, response.CodeUnauthorized)
}

func TestRequireAdminReturnsForbiddenForNonAdmin(t *testing.T) {
	middleware := RequireAdmin()
	next := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, dbsql.User{
		ID:   9,
		Role: 2,
	}))
	rec := httptest.NewRecorder()

	next.ServeHTTP(rec, req)

	assertErrorResponse(t, rec, http.StatusForbidden, response.CodeForbidden)
}

func TestRequireAdminAllowsAdminUser(t *testing.T) {
	middleware := RequireAdmin()
	nextCalled := false
	next := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/1", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, dbsql.User{
		ID:   1,
		Role: RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	next.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}
