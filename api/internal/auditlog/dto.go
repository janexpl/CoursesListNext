package auditlog

import "encoding/json"

type EntryDTO struct {
	ID             int64           `json:"id"`
	EntityType     string          `json:"entityType"`
	EntityID       int64           `json:"entityId"`
	Action         string          `json:"action"`
	ActorUserID    *int64          `json:"actorUserId"`
	ActorUserEmail *string         `json:"actorUserEmail"`
	ActorUserName  *string         `json:"actorUserName"`
	RequestID      *string         `json:"requestId"`
	Before         json.RawMessage `json:"before"`
	After          json.RawMessage `json:"after"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      string          `json:"createdAt"`
}
type ListResponse struct {
	Data []EntryDTO `json:"data"`
}
