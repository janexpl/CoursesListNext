package dashboard

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
	getDashboardStats         func(ctx context.Context) (sqlc.GetDashboardStatsRow, error)
	listExpiringCertificates  func(ctx context.Context) ([]sqlc.ListExpiringCertificatesRow, error)
	countExpiringCertificates func(ctx context.Context) (int64, error)
}

func (f fakeQuerier) GetDashboardStats(ctx context.Context) (sqlc.GetDashboardStatsRow, error) {
	if f.getDashboardStats == nil {
		return sqlc.GetDashboardStatsRow{}, errors.New("unexpected GetDashboardStats call")
	}
	return f.getDashboardStats(ctx)
}

func (f fakeQuerier) ListExpiringCertificates(ctx context.Context) ([]sqlc.ListExpiringCertificatesRow, error) {
	if f.listExpiringCertificates == nil {
		return nil, errors.New("unexpected ListExpiringCertificates call")
	}
	return f.listExpiringCertificates(ctx)
}

func (f fakeQuerier) CountExpiringCertificates(ctx context.Context) (int64, error) {
	if f.countExpiringCertificates == nil {
		return 0, errors.New("unexpected CountExpiringCertificates call")
	}
	return f.countExpiringCertificates(ctx)
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

func TestGetReturnsDashboardResponse(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getDashboardStats: func(context.Context) (sqlc.GetDashboardStatsRow, error) {
			return sqlc.GetDashboardStatsRow{
				TotalStudents:     12,
				TotalCompanies:    4,
				TotalCertificates: 87,
			}, nil
		},
		listExpiringCertificates: func(context.Context) ([]sqlc.ListExpiringCertificatesRow, error) {
			return []sqlc.ListExpiringCertificatesRow{
				{
					ID:           21,
					ExpiryDate:   "2026-04-02",
					Firstname:    "Jan",
					Lastname:     "Nowak",
					CompanyName:  "ABC Sp. z o.o.",
					Year:         2026,
					Number:       14,
					CourseName:   "Szkolenie BHP",
					CourseSymbol: "BHP",
				},
			}, nil
		},
		countExpiringCertificates: func(context.Context) (int64, error) {
			return 7, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}

	var response DashboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Data.Stats.Students != 12 || response.Data.Stats.Companies != 4 || response.Data.Stats.Certificates != 87 {
		t.Fatalf("unexpected stats payload: %+v", response.Data.Stats)
	}
	if response.Data.Expiring.In30Days != 7 {
		t.Fatalf("expected 7 expiring certificates, got %d", response.Data.Expiring.In30Days)
	}
	if len(response.Data.ExpiringCertificates) != 1 {
		t.Fatalf("expected 1 expiring certificate, got %d", len(response.Data.ExpiringCertificates))
	}

	item := response.Data.ExpiringCertificates[0]
	if item.CertificateID != 21 || item.StudentName != "Jan Nowak" || item.ExpiryDate != "2026-04-02" {
		t.Fatalf("unexpected expiring certificate payload: %+v", item)
	}
}

func TestGetReturnsInternalServerErrorWhenStatsFail(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getDashboardStats: func(context.Context) (sqlc.GetDashboardStatsRow, error) {
			return sqlc.GetDashboardStatsRow{}, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestGetReturnsInternalServerErrorWhenExpiringListFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getDashboardStats: func(context.Context) (sqlc.GetDashboardStatsRow, error) {
			return sqlc.GetDashboardStatsRow{}, nil
		},
		listExpiringCertificates: func(context.Context) ([]sqlc.ListExpiringCertificatesRow, error) {
			return nil, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}

func TestGetReturnsInternalServerErrorWhenExpiringCountFails(t *testing.T) {
	handler := NewHandler(fakeQuerier{
		getDashboardStats: func(context.Context) (sqlc.GetDashboardStatsRow, error) {
			return sqlc.GetDashboardStatsRow{}, nil
		},
		listExpiringCertificates: func(context.Context) ([]sqlc.ListExpiringCertificatesRow, error) {
			return []sqlc.ListExpiringCertificatesRow{}, nil
		},
		countExpiringCertificates: func(context.Context) (int64, error) {
			return 0, errors.New("db error")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	assertErrorResponse(t, rec, http.StatusInternalServerError, response.CodeInternalError)
}
