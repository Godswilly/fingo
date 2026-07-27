-- name: ReserveIdempotency :one
INSERT INTO idempotency_keys (key, fingerprint, status, created_at, updated_at)
VALUES ($1, $2, $3, now(), now())
ON CONFLICT (key) DO NOTHING
RETURNING key, fingerprint, status, result_ref, created_at, updated_at;

-- name: MarkIdempotencyInProgress :one
UPDATE idempotency_keys
SET status = $3, updated_at = now()
WHERE key = $1 AND fingerprint = $2 AND status = $4
RETURNING key, fingerprint, status, result_ref, created_at, updated_at;

-- name: SaveIdempotencyCompleted :execrows
UPDATE idempotency_keys
SET status = $2, result_ref = $3, updated_at = now()
WHERE key = $1;

-- name: LockIdempotencyByKey :one
SELECT key, fingerprint, status, result_ref, created_at, updated_at
FROM idempotency_keys
WHERE key = $1
FOR UPDATE;
