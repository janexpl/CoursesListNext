package apikeys

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeListQuerier struct {
	rows []sqlc.ListAPIKeysRow
	err  error
}

func (f fakeListQuerier) ListAPIKeys(context.Context) ([]sqlc.ListAPIKeysRow, error) {
	return f.rows, f.err
}

type fakeManager struct {
	create func(ctx context.Context, req CreateAPIKeyRequest) (APIKeyDTO, string, error)
	revoke func(ctx context.Context, id int64) error
}

func (f fakeManager) Create(ctx context.Context, req CreateAPIKeyRequest) (APIKeyDTO, string, error) {
	if f.create == nil {
		return APIKeyDTO{}, "", errors.New("unexpected create call")
	}
	return f.create(ctx, req)
}

func (f fakeManager) Revoke(ctx context.Context, id int64) error {
	if f.revoke == nil {
		return errors.New("unexpected revoke call")
	}
	return f.revoke(ctx, id)
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedCode string) {
	t.Helper()

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, rec.Code)
	}

	var body response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error.Code != expectedCode {
		t.Fatalf("expected error code %q, got %q", expectedCode, body.Error.Code)
	}
}

// requestWithID podkłada parametr ścieżki, który w produkcji ustawia router.
func requestWithID(method, target, id string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.SetPathValue("id", id)
	return req
}

func TestCreateReturnsRawTokenOnce(t *testing.T) {
	handler := NewHandler(fakeListQuerier{}, fakeManager{
		create: func(_ context.Context, req CreateAPIKeyRequest) (APIKeyDTO, string, error) {
			if req.Name != "integracja kadrowa" {
				t.Errorf("unexpected name %q", req.Name)
			}
			return APIKeyDTO{ID: 7, Name: req.Name, Prefix: "clk_abcd1234", Scopes: req.Scopes}, "clk_surowy-klucz", nil
		},
	})

	body := `{"name":"integracja kadrowa","userId":42,"scopes":["students:read"],"expiresAt":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/api-keys", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var resp CreateAPIKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Token != "clk_surowy-klucz" {
		t.Fatalf("expected the raw key in the create response, got %q", resp.Token)
	}
	if resp.Data.ID != 7 {
		t.Fatalf("expected the created key, got %+v", resp.Data)
	}
}

func TestCreateRejectsUnknownFields(t *testing.T) {
	handler := NewHandler(fakeListQuerier{}, fakeManager{})

	body := `{"name":"integracja","userId":42,"scopes":["students:read"],"token":"podstawiony"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/api-keys", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Create(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestCreateMapsValidationErrors(t *testing.T) {
	tests := []struct {
		name         string
		serviceError error
		wantStatus   int
		wantCode     string
	}{
		{name: "pusta nazwa", serviceError: ErrInvalidInput, wantStatus: http.StatusBadRequest, wantCode: response.CodeBadRequest},
		{name: "brak zakresów", serviceError: ErrNoScopes, wantStatus: http.StatusBadRequest, wantCode: response.CodeBadRequest},
		{name: "nieznany zakres", serviceError: ErrUnknownScope, wantStatus: http.StatusBadRequest, wantCode: response.CodeBadRequest},
		{name: "data w przeszłości", serviceError: ErrExpiryInPast, wantStatus: http.StatusBadRequest, wantCode: response.CodeBadRequest},
		{name: "brak konta", serviceError: ErrUserNotFound, wantStatus: http.StatusBadRequest, wantCode: response.CodeBadRequest},
		{name: "awaria bazy", serviceError: errors.New("boom"), wantStatus: http.StatusInternalServerError, wantCode: response.CodeInternalError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(fakeListQuerier{}, fakeManager{
				create: func(context.Context, CreateAPIKeyRequest) (APIKeyDTO, string, error) {
					return APIKeyDTO{}, "", tc.serviceError
				},
			})

			body := `{"name":"integracja","userId":42,"scopes":["students:read"]}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/api-keys", strings.NewReader(body))
			rec := httptest.NewRecorder()

			handler.Create(rec, req)

			assertErrorResponse(t, rec, tc.wantStatus, tc.wantCode)
		})
	}
}

func TestListReturnsKeysWithoutSecrets(t *testing.T) {
	handler := NewHandler(fakeListQuerier{
		rows: []sqlc.ListAPIKeysRow{{
			ID:            7,
			Name:          "integracja kadrowa",
			Prefix:        "clk_abcd1234",
			UserID:        42,
			Scopes:        []string{auth.ScopeStudentsRead},
			UserEmail:     "integracja@example.com",
			UserFirstname: "Konto",
			UserLastname:  "Serwisowe",
		}},
	}, fakeManager{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-keys", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	raw := rec.Body.String()
	if strings.Contains(raw, "token_hash") || strings.Contains(raw, `"token"`) {
		t.Fatalf("listing must not expose key material, got %s", raw)
	}

	var resp ListAPIKeysResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].Prefix != "clk_abcd1234" {
		t.Fatalf("unexpected listing payload: %+v", resp.Data)
	}
}

func TestListScopesReturnsCatalog(t *testing.T) {
	handler := NewHandler(fakeListQuerier{}, fakeManager{})

	rec := httptest.NewRecorder()
	handler.ListScopes(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-keys/scopes", nil))

	var resp ScopesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != len(auth.AssignableScopes()) {
		t.Fatalf("expected the full scope catalog, got %v", resp.Data)
	}
}

func TestRevokeReturnsNoContent(t *testing.T) {
	var revoked int64
	handler := NewHandler(fakeListQuerier{}, fakeManager{
		revoke: func(_ context.Context, id int64) error {
			revoked = id
			return nil
		},
	})

	rec := httptest.NewRecorder()
	handler.Revoke(rec, requestWithID(http.MethodDelete, "/api/v1/admin/api-keys/7", "7"))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
	if revoked != 7 {
		t.Fatalf("expected key 7 to be revoked, got %d", revoked)
	}
}

func TestRevokeAlreadyRevokedReturnsConflict(t *testing.T) {
	handler := NewHandler(fakeListQuerier{}, fakeManager{
		revoke: func(context.Context, int64) error { return ErrAlreadyRevoked },
	})

	rec := httptest.NewRecorder()
	handler.Revoke(rec, requestWithID(http.MethodDelete, "/api/v1/admin/api-keys/7", "7"))

	assertErrorResponse(t, rec, http.StatusConflict, response.CodeConflict)
}

func TestRevokeMissingKeyReturnsNotFound(t *testing.T) {
	handler := NewHandler(fakeListQuerier{}, fakeManager{
		revoke: func(context.Context, int64) error { return pgx.ErrNoRows },
	})

	rec := httptest.NewRecorder()
	handler.Revoke(rec, requestWithID(http.MethodDelete, "/api/v1/admin/api-keys/7", "7"))

	assertErrorResponse(t, rec, http.StatusNotFound, response.CodeNotFound)
}

func TestRevokeInvalidIDReturnsBadRequest(t *testing.T) {
	handler := NewHandler(fakeListQuerier{}, fakeManager{})

	rec := httptest.NewRecorder()
	handler.Revoke(rec, requestWithID(http.MethodDelete, "/api/v1/admin/api-keys/abc", "abc"))

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}
