-- name: CreateAPIKey :one
INSERT INTO api_keys (name, prefix, token_hash, user_id, scopes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, prefix, user_id, scopes, expires_at, last_used_at, revoked_at, created_at;

-- name: GetActiveAPIKeyByTokenHash :one
-- Zwraca klucz razem z kontem serwisowym, żeby uwierzytelnienie żądania
-- kosztowało jedno zapytanie. Klucze odwołane i wygasłe są odfiltrowane tutaj,
-- więc middleware nie musi ich rozróżniać.
SELECT k.id,
       k.name,
       k.prefix,
       k.scopes,
       k.expires_at,
       k.last_used_at,
       u.id AS user_id,
       u.email,
       u.password,
       u.firstname,
       u.lastname,
       u.role
FROM api_keys k
JOIN users u ON u.id = k.user_id
WHERE k.token_hash = $1
  AND k.revoked_at IS NULL
  AND (k.expires_at IS NULL OR k.expires_at > now());

-- name: TouchAPIKeyLastUsed :exec
-- Znacznik ostatniego użycia jest odświeżany co najwyżej raz na 5 minut, żeby
-- ruch z klucza nie generował zapisu przy każdym żądaniu.
UPDATE api_keys
SET last_used_at = now()
WHERE id = $1
  AND (last_used_at IS NULL OR last_used_at < now() - interval '5 minutes');

-- name: ListAPIKeys :many
SELECT k.id,
       k.name,
       k.prefix,
       k.user_id,
       k.scopes,
       k.expires_at,
       k.last_used_at,
       k.revoked_at,
       k.created_at,
       u.email AS user_email,
       u.firstname AS user_firstname,
       u.lastname AS user_lastname
FROM api_keys k
JOIN users u ON u.id = k.user_id
ORDER BY k.revoked_at IS NOT NULL, k.created_at DESC;

-- name: GetAPIKeyByID :one
SELECT id, name, prefix, user_id, scopes, expires_at, last_used_at, revoked_at, created_at
FROM api_keys
WHERE id = $1;

-- name: RevokeAPIKey :execrows
UPDATE api_keys
SET revoked_at = now()
WHERE id = $1
  AND revoked_at IS NULL;
