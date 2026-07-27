-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (type, aggregate_id, payload, occurred_at)
VALUES ($1, $2, $3, $4);
