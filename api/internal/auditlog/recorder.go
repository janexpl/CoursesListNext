// Package auditlog
package auditlog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-chi/chi/middleware"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/janexpl/CoursesListNext/api/internal/auth"
	"github.com/janexpl/CoursesListNext/api/internal/db/sqlc"
)

var (
	ErrCreateAuditLog = errors.New("failed to create log")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrMarshallData   = errors.New("failed to marshal before/after/metadata")
)

type Entry struct {
	EntityType string
	EntityID   int64
	Action     string
	Before     any
	After      any
	Metadata   any
}

type Recorder struct{}

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (r *Recorder) Record(ctx context.Context, queries *sqlc.Queries, entry Entry) error {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return ErrUnauthorized
	}
	requestID := middleware.GetReqID(ctx)
	beforeJSON, err := json.Marshal(entry.Before)
	if err != nil {
		return ErrMarshallData
	}
	afterJSON, err := json.Marshal(entry.After)
	if err != nil {
		return ErrMarshallData
	}
	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return ErrMarshallData
	}

	_, err = queries.CreateAuditLog(ctx, sqlc.CreateAuditLogParams{
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		Action:     entry.Action,
		ActorUserID: pgtype.Int8{
			Int64: user.ID,
			Valid: true,
		},
		ActorUserEmailSnapshot: pgtype.Text{String: user.Email, Valid: true},
		ActorUserNameSnapshot:  pgtype.Text{String: user.Firstname + " " + user.Lastname, Valid: true},
		RequestID:              pgtype.Text{String: requestID, Valid: true},
		BeforeData:             []byte(beforeJSON),
		AfterData:              []byte(afterJSON),
		Metadata:               []byte(metadataJSON),
	})
	if err != nil {
		return ErrCreateAuditLog
	}
	return nil
}
