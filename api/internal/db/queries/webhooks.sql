-- name: AcquireWebhookSubjectLock :exec
-- Szereguje zapis zdarzeń jednego przedmiotu (zaświadczenia, kursu) do końca transakcji,
-- dzięki czemu znaczniki czasu rosną w kolejności zatwierdzania.
SELECT pg_advisory_xact_lock(hashtextextended('webhook:' || @subject_key::text, 0));

-- name: NextWebhookEventTimestamp :one
-- Bieżący czas, ale ściśle późniejszy od ostatniego zdarzenia tego samego przedmiotu.
SELECT GREATEST(
    clock_timestamp(),
    COALESCE(
        (SELECT max(occurred_at) FROM webhook_events WHERE subject_key = @subject_key::text),
        '-infinity'::timestamptz
    ) + interval '1 microsecond'
)::timestamptz AS occurred_at;

-- name: InsertWebhookEvent :one
-- Zapisuje zdarzenie i doręczenia do wszystkich aktywnych odbiorców.
WITH ev AS (
    INSERT INTO webhook_events (event_type, subject_key, occurred_at, payload)
    VALUES (@event_type::text, @subject_key::text, @occurred_at::timestamptz, @payload::bytea)
    RETURNING id, subject_key
),
deliveries AS (
    INSERT INTO webhook_deliveries (event_id, endpoint_id, subject_key)
    SELECT ev.id, en.id, ev.subject_key
    FROM ev
    CROSS JOIN webhook_endpoints en
    WHERE en.active
    RETURNING id
)
SELECT (SELECT id FROM ev)::bigint AS event_id, (SELECT count(*) FROM deliveries)::bigint AS deliveries;

-- name: NotifyWebhookDispatcher :exec
-- NOTIFY w transakcji dociera do nasłuchujących dopiero po jej zatwierdzeniu.
SELECT pg_notify('webhook_deliveries', '');

-- name: ClaimWebhookDeliveries :many
-- Pobiera doręczenia gotowe do wysyłki i zakłada na nie dzierżawę. Doręczenie czeka, dopóki
-- starsze doręczenie tego samego przedmiotu do tego samego odbiorcy jest w toku - odbiorca
-- dostaje zdarzenia o dokumencie w kolejności ich powstania.
WITH due AS (
    SELECT d.id
    FROM webhook_deliveries d
    WHERE d.status = 'pending'
      AND d.next_attempt_at <= now()
      AND (d.locked_until IS NULL OR d.locked_until < now())
      AND NOT EXISTS (
          SELECT 1
          FROM webhook_deliveries older
          WHERE older.endpoint_id = d.endpoint_id
            AND older.subject_key = d.subject_key
            AND older.status = 'pending'
            AND older.id < d.id
      )
    ORDER BY d.next_attempt_at, d.id
    LIMIT sqlc.arg(batch_size)
    FOR UPDATE OF d SKIP LOCKED
)
UPDATE webhook_deliveries d
SET locked_until = now() + make_interval(secs => sqlc.arg(lease_seconds)::double precision),
    attempts = d.attempts + 1
FROM due, webhook_events ev, webhook_endpoints en
WHERE d.id = due.id
  AND ev.id = d.event_id
  AND en.id = d.endpoint_id
RETURNING d.id, d.attempts, ev.event_type, ev.payload, en.id AS endpoint_id, en.url, en.secret;

-- name: MarkWebhookDelivered :exec
UPDATE webhook_deliveries
SET status = 'delivered',
    delivered_at = now(),
    locked_until = NULL,
    last_status_code = sqlc.arg(status_code),
    last_error = NULL
WHERE id = sqlc.arg(id);

-- name: MarkWebhookRetry :exec
UPDATE webhook_deliveries
SET next_attempt_at = now() + make_interval(secs => sqlc.arg(delay_seconds)::double precision),
    locked_until = NULL,
    last_status_code = sqlc.narg(status_code),
    last_error = sqlc.narg(last_error)
WHERE id = sqlc.arg(id);

-- name: MarkWebhookFailed :exec
UPDATE webhook_deliveries
SET status = 'failed',
    locked_until = NULL,
    last_status_code = sqlc.narg(status_code),
    last_error = sqlc.narg(last_error)
WHERE id = sqlc.arg(id);
