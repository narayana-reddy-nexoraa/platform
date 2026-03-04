-- name: CreateExecution :one
INSERT INTO executions (
    tenant_id, idempotency_key, status, attempt_count, max_attempts,
    payload, payload_hash, version
) VALUES (
    $1, $2, 'CREATED', 0, $3, $4, $5, 1
)
ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetExecutionByTenantAndIdempotencyKey :one
SELECT * FROM executions
WHERE tenant_id = $1 AND idempotency_key = $2;

-- name: GetExecutionByID :one
SELECT * FROM executions
WHERE execution_id = $1 AND tenant_id = $2;

-- name: ListExecutions :many
SELECT * FROM executions
WHERE tenant_id = $1
  AND (sqlc.narg('status')::execution_status IS NULL OR status = sqlc.narg('status')::execution_status)
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountExecutions :one
SELECT COUNT(*) FROM executions
WHERE tenant_id = $1
  AND (sqlc.narg('status')::execution_status IS NULL OR status = sqlc.narg('status')::execution_status);

-- name: FindClaimableExecutions :many
SELECT * FROM executions
WHERE status IN ('CREATED', 'FAILED')
  AND (locked_by IS NULL OR lock_expires_at < NOW())
  AND attempt_count < max_attempts
  AND (retry_after IS NULL OR retry_after <= NOW())
ORDER BY created_at ASC
LIMIT $1;

-- name: ClaimExecution :one
UPDATE executions
SET status = 'CLAIMED',
    locked_by = $2,
    lock_expires_at = NOW() + INTERVAL '1 second' * $3,
    attempt_count = attempt_count + 1,
    version = version + 1
WHERE execution_id = $1
  AND version = $4
  AND (status IN ('CREATED', 'FAILED'))
  AND (locked_by IS NULL OR lock_expires_at < NOW())
RETURNING *;

-- name: UpdateExecutionStatus :one
UPDATE executions
SET status = $2,
    version = version + 1
WHERE execution_id = $1
  AND version = $3
RETURNING *;

-- name: CompleteExecution :one
UPDATE executions
SET status = 'SUCCEEDED',
    locked_by = NULL,
    lock_expires_at = NULL,
    version = version + 1
WHERE execution_id = $1
  AND version = $2
RETURNING *;

-- name: FailExecution :one
UPDATE executions
SET status = 'FAILED',
    locked_by = NULL,
    lock_expires_at = NULL,
    error_code = $2,
    error_message = $3,
    version = version + 1
WHERE execution_id = $1
  AND version = $4
RETURNING *;

-- name: SendHeartbeat :one
UPDATE executions
SET lock_expires_at = NOW() + INTERVAL '1 second' * $2,
    last_heartbeat_at = NOW(),
    version = version + 1
WHERE execution_id = $1
  AND locked_by = $3
  AND status IN ('CLAIMED', 'RUNNING')
RETURNING *;

-- name: FindExpiredLeases :many
SELECT * FROM executions
WHERE locked_by IS NOT NULL
  AND lock_expires_at < NOW()
  AND status IN ('CLAIMED', 'RUNNING')
ORDER BY lock_expires_at ASC
LIMIT $1;

-- name: ReclaimExecution :one
UPDATE executions
SET status = 'CREATED',
    locked_by = NULL,
    lock_expires_at = NULL,
    last_heartbeat_at = NULL,
    version = version + 1
WHERE execution_id = $1
  AND version = $2
  AND lock_expires_at < NOW()
RETURNING *;

-- name: CountActiveExecutions :one
SELECT COUNT(*) FROM executions
WHERE status IN ('CLAIMED', 'RUNNING');

-- name: CountPendingExecutions :one
SELECT COUNT(*) FROM executions
WHERE status = 'CREATED';

-- name: RetryExecution :one
UPDATE executions
SET status = 'CREATED',
    locked_by = NULL,
    lock_expires_at = NULL,
    last_heartbeat_at = NULL,
    error_code = $2,
    error_message = $3,
    retry_after = NOW() + ($4 * INTERVAL '1 millisecond'),
    version = version + 1
WHERE execution_id = $1
  AND version = $5
RETURNING *;
