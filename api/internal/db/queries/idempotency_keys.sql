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
-- Klucz trafia też na zaświadczenie (certificates.idempotency_key), bo wpis w tej tabeli
-- jest usuwany po 30 dniach, a zdarzenia webhooka o dokumencie mogą przyjść później.
WITH stored AS (
    INSERT INTO idempotency_keys (key, request_hash, certificate_id)
    VALUES (@key::text, @request_hash::text, @certificate_id::bigint)
    RETURNING key, certificate_id
)
UPDATE certificates
SET idempotency_key = stored.key
FROM stored
WHERE certificates.id = stored.certificate_id;

-- name: DeleteIdempotencyKeysOlderThan :execrows
DELETE FROM idempotency_keys
WHERE created_at < @cutoff::timestamptz;
