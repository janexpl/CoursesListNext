package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeQuerier struct {
	listAuditLogsByEntityFunc func(ctx context.Context, arg sqlc.ListAuditLogsByEntityParams) ([]sqlc.AuditLog, error)
}

func (f fakeQuerier) ListAuditLogsByEntity(ctx context.Context, arg sqlc.ListAuditLogsByEntityParams) ([]sqlc.AuditLog, error) {
	if f.listAuditLogsByEntityFunc == nil {
		return nil, errors.New("unexpected ListAuditLogsByEntity call")
	}
	return f.listAuditLogsByEntityFunc(ctx, arg)
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

func TestListByEntityReturnsAuditEntries(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listAuditLogsByEntityFunc: func(_ context.Context, arg sqlc.ListAuditLogsByEntityParams) ([]sqlc.AuditLog, error) {
			if arg.EntityType != "course" || arg.EntityID != 12 {
				t.Fatalf("unexpected query params: %+v", arg)
			}

			return []sqlc.AuditLog{
				{
					ID:                     101,
					EntityType:             "course",
					EntityID:               12,
					Action:                 "update",
					ActorUserID:            pgtype.Int8{Int64: 9, Valid: true},
					ActorUserEmailSnapshot: pgtype.Text{String: "admin@example.com", Valid: true},
					ActorUserNameSnapshot:  pgtype.Text{String: "Admin User", Valid: true},
					RequestID:              pgtype.Text{String: "req-123", Valid: true},
					BeforeData:             []byte(`{"name":"before"}`),
					AfterData:              []byte(`{"name":"after"}`),
					Metadata:               []byte(`{"source":"manual"}`),
					CreatedAt:              pgtype.Timestamptz{Time: time.Date(2026, time.March, 28, 12, 30, 0, 0, time.UTC), Valid: true},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/courses/12/audit-log", nil)
	req.SetPathValue("id", "12")
	rec := httptest.NewRecorder()

	handler.ListByEntity("course")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(responseBody.Data))
	}

	entry := responseBody.Data[0]
	if entry.ID != 101 || entry.EntityType != "course" || entry.EntityID != 12 || entry.Action != "update" {
		t.Fatalf("unexpected audit entry payload: %+v", entry)
	}
	if entry.ActorUserID == nil || *entry.ActorUserID != 9 {
		t.Fatalf("expected actor user id 9, got %+v", entry.ActorUserID)
	}
	if entry.ActorUserEmail == nil || *entry.ActorUserEmail != "admin@example.com" {
		t.Fatalf("expected actor user email, got %+v", entry.ActorUserEmail)
	}
	if entry.ActorUserName == nil || *entry.ActorUserName != "Admin User" {
		t.Fatalf("expected actor user name, got %+v", entry.ActorUserName)
	}
	if entry.RequestID == nil || *entry.RequestID != "req-123" {
		t.Fatalf("expected request id, got %+v", entry.RequestID)
	}
	if string(entry.Before) != `{"name":"before"}` {
		t.Fatalf("unexpected before payload: %s", string(entry.Before))
	}
	if string(entry.After) != `{"name":"after"}` {
		t.Fatalf("unexpected after payload: %s", string(entry.After))
	}
	if string(entry.Metadata) != `{"source":"manual"}` {
		t.Fatalf("unexpected metadata payload: %s", string(entry.Metadata))
	}
	if entry.CreatedAt != "2026-03-28 12:30:00" {
		t.Fatalf("unexpected createdAt value: %q", entry.CreatedAt)
	}
}

func TestListByEntityReturnsNullJSONForEmptyFields(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listAuditLogsByEntityFunc: func(_ context.Context, arg sqlc.ListAuditLogsByEntityParams) ([]sqlc.AuditLog, error) {
			return []sqlc.AuditLog{
				{
					ID:         102,
					EntityType: arg.EntityType,
					EntityID:   arg.EntityID,
					Action:     "create",
					CreatedAt:  pgtype.Timestamptz{Time: time.Date(2026, time.March, 28, 14, 0, 0, 0, time.UTC), Valid: true},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/companies/15/audit-log", nil)
	req.SetPathValue("id", "15")
	rec := httptest.NewRecorder()

	handler.ListByEntity("company")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	entry := responseBody.Data[0]
	if string(entry.Before) != "null" || string(entry.After) != "null" || string(entry.Metadata) != "null" {
		t.Fatalf("expected empty json fields to map to null, got before=%s after=%s metadata=%s", string(entry.Before), string(entry.After), string(entry.Metadata))
	}
	if entry.ActorUserID != nil || entry.ActorUserEmail != nil || entry.ActorUserName != nil || entry.RequestID != nil {
		t.Fatalf("expected nil optional actor fields, got %+v", entry)
	}
}

func TestListByEntityReturnsEmptyList(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listAuditLogsByEntityFunc: func(_ context.Context, arg sqlc.ListAuditLogsByEntityParams) ([]sqlc.AuditLog, error) {
			return []sqlc.AuditLog{}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/students/7/audit-log", nil)
	req.SetPathValue("id", "7")
	rec := httptest.NewRecorder()

	handler.ListByEntity("student")(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var responseBody ListResponse
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(responseBody.Data) != 0 {
		t.Fatalf("expected empty audit log list, got %+v", responseBody.Data)
	}
}

func TestListByEntityReturnsBadRequestForInvalidID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listAuditLogsByEntityFunc: func(_ context.Context, arg sqlc.ListAuditLogsByEntityParams) ([]sqlc.AuditLog, error) {
			t.Fatalf("ListAuditLogsByEntity should not be called for invalid id, got %+v", arg)
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/courses/abc/audit-log", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.ListByEntity("course")(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestListByEntityReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		listAuditLogsByEntityFunc: func(_ context.Context, arg sqlc.ListAuditLogsByEntityParams) ([]sqlc.AuditLog, error) {
			return nil, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/courses/12/audit-log", nil)
	req.SetPathValue("id", "12")
	rec := httptest.NewRecorder()

	handler.ListByEntity("course")(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}
