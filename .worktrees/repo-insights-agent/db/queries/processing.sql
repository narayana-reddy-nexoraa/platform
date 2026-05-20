-- name: InsertProcessedEvent :one
INSERT INTO processed_events (event_id, consumer_group)
VALUES ($1, $2)
ON CONFLICT (event_id, consumer_group) DO NOTHING
RETURNING event_id;

-- name: InsertProcessingLog :exec
INSERT INTO processing_log (execution_id, worker_id, action, attempt_number)
VALUES ($1, $2, $3, $4);

-- name: CountCompletedByExecution :many
SELECT execution_id, COUNT(*) as completion_count
FROM processing_log
WHERE action = 'COMPLETED'
GROUP BY execution_id
HAVING COUNT(*) > 1;

-- name: ListProcessingLogByExecution :many
SELECT * FROM processing_log
WHERE execution_id = $1
ORDER BY created_at ASC;
