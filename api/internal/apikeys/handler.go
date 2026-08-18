package apikeys

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	ListAPIKeys(ctx context.Context) ([]sqlc.ListAPIKeysRow, error)
}

type Manager interface {
	Create(ctx context.Context, req CreateAPIKeyRequest) (APIKeyDTO, string, error)
	Revoke(ctx context.Context, id int64) error
}

type Handler struct {
	querier Querier
	manager Manager
}

func NewHandler(querier Querier, manager Manager) *Handler {
	return &Handler{
		querier: querier,
		manager: manager,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.querier.ListAPIKeys(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to retrieve api keys")
		return
	}
	resp := ListAPIKeysResponse{Data: make([]APIKeyDTO, 0, len(rows))}
	for _, row := range rows {
		resp.Data = append(resp.Data, mapListRow(row))
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

// ListScopes zwraca katalog zakresów, żeby panel admina nie musiał trzymać
// własnej kopii listy, która rozjedzie się z backendem.
func (h *Handler) ListScopes(w http.ResponseWriter, _ *http.Request) {
	response.WriteJSON(w, http.StatusOK, ScopesResponse{Data: auth.AssignableScopes()})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	req := CreateAPIKeyRequest{}
	if err := decoder.Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	dto, token, err := h.manager.Create(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		case errors.Is(err, ErrNoScopes):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "at least one scope is required")
		case errors.Is(err, ErrUnknownScope):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "unknown scope")
		case errors.Is(err, ErrExpiryInPast):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "expiry date must be in the future")
		case errors.Is(err, ErrUserNotFound):
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "service account not found")
		default:
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to create api key")
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, CreateAPIKeyResponse{Data: dto, Token: token})
}

func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := response.ParsePositiveInt64PathValue(r, "id")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid api key id")
		return
	}

	if err := h.manager.Revoke(r.Context(), id); err != nil {
		if errors.Is(err, ErrAlreadyRevoked) {
			response.WriteError(w, http.StatusConflict, response.CodeConflict, "api key is already revoked")
			return
		}
		response.HandleDBError(w, err, "api key")
		return
	}

	response.WriteNoContent(w)
}
