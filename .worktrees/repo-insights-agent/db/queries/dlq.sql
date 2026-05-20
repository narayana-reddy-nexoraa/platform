-- name: InsertDLQEvent :exec
INSERT INTO dead_letter_events (
    event_id, consumer_group, event_type, aggregate_type, aggregate_id,
    payload, metadata, error_message, attempt_count, max_attempts
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListDLQEvents :many
SELECT * FROM dead_letter_events
WHERE consumer_group = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeleteDLQEvent :exec
DELETE FROM dead_letter_events WHERE id = $1;

-- name: CountDLQEvents :one
SELECT COUNT(*) FROM dead_letter_events WHERE consumer_group = $1;

-- name: GetDLQEventByEventID :one
SELECT * FROM dead_letter_events WHERE event_id = $1 AND consumer_group = $2;
