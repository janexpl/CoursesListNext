-- name: CreateAuditLog :one
INSERT INTO audit_log (
    entity_type,
    entity_id,
    action,
    actor_user_id,
    actor_user_email_snapshot,
    actor_user_name_snapshot,
    request_id,
    before_data,
    after_data,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id;
-- name: ListAuditLogsByEntity :many
SELECT
    id,
    entity_type,
    entity_id,
    action,
    actor_user_id,
    actor_user_email_snapshot,
    actor_user_name_snapshot,
    request_id,
    before_data,
    after_data,
    metadata,
    created_at
FROM audit_log
WHERE entity_type = $1
  AND entity_id = $2
ORDER BY created_at DESC, id DESC;
