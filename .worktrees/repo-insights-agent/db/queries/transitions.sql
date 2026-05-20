-- name: InsertTransition :exec
INSERT INTO execution_transitions (
    execution_id, from_status, to_status, triggered_by, reason, metadata
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListTransitionsByExecution :many
SELECT * FROM execution_transitions
WHERE execution_id = $1
ORDER BY created_at ASC;
