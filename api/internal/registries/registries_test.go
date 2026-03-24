package registries

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type fakeQuerier struct {
	getNextRegistryNumberFunc func(ctx context.Context, arg sqlc.GetNextRegistryNumberParams) (int32, error)
}

func (f fakeQuerier) GetNextRegistryNumber(ctx context.Context, arg sqlc.GetNextRegistryNumberParams) (int32, error) {
	if f.getNextRegistryNumberFunc == nil {
		return 0, errors.New("unexpected GetNextRegistryNumber call")
	}
	return f.getNextRegistryNumberFunc(ctx, arg)
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

func TestGetNextNumberReturnsResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getNextRegistryNumberFunc: func(_ context.Context, arg sqlc.GetNextRegistryNumberParams) (int32, error) {
			if arg.CourseID != 3 || arg.Year != 2026 {
				t.Fatalf("unexpected params: %+v", arg)
			}
			return 18, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/registries/next-number?courseId=3&year=2026", nil)
	rec := httptest.NewRecorder()

	handler.GetNextNumber(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var responseBody ResponseNumber
	if err := json.NewDecoder(rec.Body).Decode(&responseBody); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if responseBody.Data.CourseID != 3 || responseBody.Data.Year != 2026 || responseBody.Data.NextNumber != 18 {
		t.Fatalf("unexpected response payload: %+v", responseBody.Data)
	}
}

func TestGetNextNumberReturnsBadRequestForInvalidCourseID(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getNextRegistryNumberFunc: func(_ context.Context, arg sqlc.GetNextRegistryNumberParams) (int32, error) {
			t.Fatalf("GetNextRegistryNumber should not be called for invalid courseId, got %+v", arg)
			return 0, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/registries/next-number?courseId=abc&year=2026", nil)
	rec := httptest.NewRecorder()

	handler.GetNextNumber(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetNextNumberReturnsBadRequestForInvalidYear(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getNextRegistryNumberFunc: func(_ context.Context, arg sqlc.GetNextRegistryNumberParams) (int32, error) {
			t.Fatalf("GetNextRegistryNumber should not be called for invalid year, got %+v", arg)
			return 0, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/registries/next-number?courseId=3&year=bad", nil)
	rec := httptest.NewRecorder()

	handler.GetNextNumber(rec, req)

	assertErrorResponse(t, rec, http.StatusBadRequest, response.CodeBadRequest)
}

func TestGetNextNumberReturnsInternalServerErrorWhenQueryFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getNextRegistryNumberFunc: func(_ context.Context, arg sqlc.GetNextRegistryNumberParams) (int32, error) {
			return 0, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/registries/next-number?courseId=3&year=2026", nil)
	rec := httptest.NewRecorder()

	handler.GetNextNumber(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}
