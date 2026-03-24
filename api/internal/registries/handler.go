package registries

import (
	"context"
	"net/http"
	"strconv"

	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	GetNextRegistryNumber(ctx context.Context, arg sqlc.GetNextRegistryNumberParams) (int32, error)
}

type Handler struct {
	querier Querier
}

func NewHandler(querier Querier) *Handler {
	return &Handler{querier: querier}
}

func (h *Handler) GetNextNumber(w http.ResponseWriter, r *http.Request) {
	courseID, err := strconv.ParseInt(r.URL.Query().Get("courseId"), 10, 64)
	if err != nil || courseID <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid course id")
		return
	}

	year, err := strconv.ParseInt(r.URL.Query().Get("year"), 10, 64)
	if err != nil || year <= 0 {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid year")
		return
	}

	number, err := h.querier.GetNextRegistryNumber(r.Context(), sqlc.GetNextRegistryNumberParams{
		CourseID: courseID,
		Year:     year,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to get next registry number")
		return
	}
	response.WriteJSON(w, http.StatusOK, ResponseNumber{Data: RegistryNumberDTO{
		CourseID:   courseID,
		Year:       year,
		NextNumber: int64(number),
	}})
}
