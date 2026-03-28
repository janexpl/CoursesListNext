package auditlog

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
	"github.com/janexpl/CoursesListNext/api/internal/pgutil"
	"github.com/janexpl/CoursesListNext/api/internal/response"
)

type Querier interface {
	ListAuditLogsByEntity(ctx context.Context, arg sqlc.ListAuditLogsByEntityParams) ([]sqlc.AuditLog, error)
}

type Handler struct {
	queriers Querier
}

func NewHandler(queriers Querier) *Handler {
	return &Handler{
		queriers: queriers,
	}
}

func (h *Handler) ListByEntity(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := response.ParsePositiveInt64PathValue(r, "id")
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
			return
		}

		rows, err := h.queriers.ListAuditLogsByEntity(r.Context(), sqlc.ListAuditLogsByEntityParams{
			EntityType: entityType,
			EntityID:   id,
		})
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, response.CodeInternalError, "failed to list audit logs")
			return
		}
		resp := ListResponse{Data: make([]EntryDTO, 0, len(rows))}
		for _, row := range rows {
			resp.Data = append(resp.Data, mapEntry(row))
		}
		response.WriteJSON(w, http.StatusOK, resp)
	}
}

func mapEntry(row sqlc.AuditLog) EntryDTO {
	return EntryDTO{
		ID:             row.ID,
		EntityType:     row.EntityType,
		EntityID:       row.EntityID,
		Action:         row.Action,
		ActorUserID:    pgutil.NullableInt64(row.ActorUserID),
		ActorUserEmail: pgutil.NullableString(row.ActorUserEmailSnapshot),
		ActorUserName:  pgutil.NullableString(row.ActorUserNameSnapshot),
		RequestID:      pgutil.NullableString(row.RequestID),
		Before:         normalizeRawJSON(row.BeforeData),
		After:          normalizeRawJSON(row.AfterData),
		Metadata:       normalizeRawJSON(row.Metadata),
		CreatedAt:      row.CreatedAt.Time.Format(response.TimestampzFormat),
	}
}

func normalizeRawJSON(value []byte) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(value)
}
