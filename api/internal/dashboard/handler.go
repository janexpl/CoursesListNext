package dashboard

import (
	"context"
	"net/http"

	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	GetDashboardStats(ctx context.Context) (sqlc.GetDashboardStatsRow, error)
	ListExpiringCertificates(ctx context.Context) ([]sqlc.ListExpiringCertificatesRow, error)
	CountExpiringCertificates(ctx context.Context) (int64, error)
}

type Handler struct {
	queries Querier
}

func NewHandler(queries Querier) *Handler {
	return &Handler{
		queries: queries,
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	statsRow, err := h.queries.GetDashboardStats(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to get dashboard stats")
		return
	}
	expiringRows, err := h.queries.ListExpiringCertificates(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list expiring certificates")
		return
	}
	expiringCount, err := h.queries.CountExpiringCertificates(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to count expiring certificates")
		return
	}
	resp := DashboardResponse{
		Data: DashboardDataDTO{
			Stats: DashboardStatsDTO{
				Students:     statsRow.TotalStudents,
				Companies:    statsRow.TotalCompanies,
				Certificates: statsRow.TotalCertificates,
			},
			Expiring: ExpiringSummaryDTO{
				In30Days: int(expiringCount),
			},
			ExpiringCertificates: mapExpiringCertificates(expiringRows),
		},
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

func mapExpiringCertificates(rows []sqlc.ListExpiringCertificatesRow) []ExpiringCertificateDTO {
	dtos := make([]ExpiringCertificateDTO, 0, len(rows))
	for _, row := range rows {
		studentName := row.Firstname + " " + row.Lastname
		dtos = append(dtos, ExpiringCertificateDTO{
			CertificateID:  row.ID,
			StudentName:    studentName,
			CourseName:     row.CourseName,
			ExpiryDate:     row.ExpiryDate,
			CompanyName:    row.CompanyName,
			CourseSymbol:   row.CourseSymbol,
			RegistryYear:   row.Year,
			RegistryNumber: float64(row.Number),
		})
	}
	return dtos
}
