-- name: GetUserByEmail :one
SELECT id, email, password, firstname, lastname, role FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, password, firstname, lastname, role FROM users WHERE id = $1;


-- name: CreateSession :one
INSERT INTO api_sessions (token, user_id, expires_at, created_at)
VALUES (
    $1, $2, $3, NOW()
)
RETURNING token, user_id, expires_at, created_at;

-- name: GetSessionByToken :one
SELECT token, user_id, expires_at, created_at FROM api_sessions WHERE token = $1;

-- name: DeleteSessionByToken :exec
DELETE FROM api_sessions WHERE token = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM api_sessions WHERE user_id = $1;


