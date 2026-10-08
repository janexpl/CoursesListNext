package legalbases

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	detailsReader
	ListLegalBases(ctx context.Context) ([]sqlc.ListLegalBasesRow, error)
}

type Writer interface {
	Create(ctx context.Context, input Input) (LegalBasisDetailsDTO, error)
	Update(ctx context.Context, id int64, input Input) (LegalBasisDetailsDTO, error)
	Delete(ctx context.Context, id int64) error
}

type Handler struct {
	queries Querier
	writer  Writer
}

func NewHandler(queries Querier, writer Writer) *Handler {
	return &Handler{queries: queries, writer: writer}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListLegalBases(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list legal bases")
		return
	}
	out := make([]LegalBasisDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, makeDTO(sqlc.LegalBasis{
			ID: row.ID, Name: row.Name, Content: row.Content, UpdatedAt: row.UpdatedAt,
		}, row.CourseCount))
	}
	response.WriteJSON(w, http.StatusOK, ListLegalBasesResponse{Data: out})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid legal basis ID")
		return
	}
	details, err := loadDetails(r.Context(), h.queries, id)
	if err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, LegalBasisResponse{Data: details})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeInput(w, r)
	if !ok {
		return
	}
	created, err := h.writer.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusCreated, LegalBasisResponse{Data: created})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid legal basis ID")
		return
	}
	input, ok := decodeInput(w, r)
	if !ok {
		return
	}
	updated, err := h.writer.Update(r.Context(), id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, LegalBasisResponse{Data: updated})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid legal basis ID")
		return
	}
	if err := h.writer.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	response.WriteNoContent(w)
}

func decodeInput(w http.ResponseWriter, r *http.Request) (Input, bool) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	req := LegalBasisRequest{}
	if err := decoder.Decode(&req); err != nil || req.Name == nil || req.Content == nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return Input{}, false
	}
	return Input{Name: *req.Name, Content: *req.Content}, true
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
	case errors.Is(err, ErrNotFound):
		response.WriteError(w, http.StatusNotFound, response.CodeNotFound, "legal basis not found")
	case errors.Is(err, ErrNameTaken):
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "legal basis name already exists")
	case errors.Is(err, ErrInUse):
		response.WriteError(w, http.StatusConflict, response.CodeConflict, "legal basis in use")
	default:
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to save legal basis")
	}
}
