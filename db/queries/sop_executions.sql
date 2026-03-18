-- name: CreateSOPExecution :one
INSERT INTO sop_executions (
    sop_id, tenant_id, industry, current_step, status,
    input_payload, temporal_workflow_id, temporal_run_id, version
) VALUES (
    $1, $2, $3, 'intake', 'PENDING', $4, $5, $6, 1
)
RETURNING *;

-- name: GetSOPExecutionByID :one
SELECT * FROM sop_executions
WHERE sop_execution_id = $1 AND tenant_id = $2;

-- name: GetSOPExecutionByWorkflowID :one
SELECT * FROM sop_executions
WHERE temporal_workflow_id = $1;

-- name: ListSOPExecutions :many
SELECT * FROM sop_executions
WHERE tenant_id = $1
  AND (sqlc.narg('sop_id')::VARCHAR IS NULL OR sop_id = sqlc.narg('sop_id')::VARCHAR)
  AND (sqlc.narg('status')::sop_execution_status IS NULL OR status = sqlc.narg('status')::sop_execution_status)
  AND (sqlc.narg('industry')::sop_industry IS NULL OR industry = sqlc.narg('industry')::sop_industry)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSOPExecutions :one
SELECT COUNT(*) FROM sop_executions
WHERE tenant_id = $1
  AND (sqlc.narg('sop_id')::VARCHAR IS NULL OR sop_id = sqlc.narg('sop_id')::VARCHAR)
  AND (sqlc.narg('status')::sop_execution_status IS NULL OR status = sqlc.narg('status')::sop_execution_status)
  AND (sqlc.narg('industry')::sop_industry IS NULL OR industry = sqlc.narg('industry')::sop_industry);

-- name: UpdateSOPExecutionStatus :one
UPDATE sop_executions
SET status = $2,
    current_step = $3,
    version = version + 1
WHERE sop_execution_id = $1
  AND version = $4
RETURNING *;

-- name: CompleteSOPExecution :one
UPDATE sop_executions
SET status = 'COMPLETED',
    output_payload = $2,
    completed_at = NOW(),
    version = version + 1
WHERE sop_execution_id = $1
  AND version = $3
RETURNING *;

-- name: FailSOPExecution :one
UPDATE sop_executions
SET status = 'FAILED',
    completed_at = NOW(),
    version = version + 1
WHERE sop_execution_id = $1
  AND version = $2
RETURNING *;
