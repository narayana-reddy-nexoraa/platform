-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (
    aggregate_type, aggregate_id, event_type, payload, metadata
) VALUES ($1, $2, $3, $4, $5);

-- name: FetchUnsentEvents :many
SELECT * FROM outbox_events
WHERE sent = FALSE
ORDER BY sequence_number ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkEventsSent :exec
UPDATE outbox_events
SET sent = TRUE, sent_at = NOW()
WHERE event_id = ANY($1::uuid[]);

-- name: CleanupOldEvents :exec
DELETE FROM outbox_events
WHERE sent = TRUE AND sent_at < $1;

-- name: CountUnsentEvents :one
SELECT COUNT(*) FROM outbox_events WHERE sent = FALSE;

-- name: ListOutboxEventsByAggregate :many
SELECT * FROM outbox_events
WHERE aggregate_type = $1 AND aggregate_id = $2
ORDER BY sequence_number ASC;
