-- name: AcquireIdempotencyKeyLock :exec
-- Szeregowanie równoległych ponowień z tym samym kluczem: druga transakcja czeka,
-- aż pierwsza zatwierdzi zaświadczenie, i dopiero wtedy sprawdza klucz.
SELECT pg_advisory_xact_lock(hashtextextended('idempotency:' || @key::text, 0));

-- name: GetIdempotencyKey :one
SELECT
    ik.request_hash,
    ik.certificate_id,
    r.year AS registry_year,
    r.number AS registry_number,
    c.verification_code
FROM idempotency_keys ik
JOIN certificates c ON c.id = ik.certificate_id
JOIN registries r ON r.id = c.registry_id
WHERE ik.key = $1;

-- name: CreateIdempotencyKey :exec
INSERT INTO idempotency_keys (key, request_hash, certificate_id)
VALUES ($1, $2, $3);

-- name: DeleteIdempotencyKeysOlderThan :execrows
DELETE FROM idempotency_keys
WHERE created_at < @cutoff::timestamptz;
