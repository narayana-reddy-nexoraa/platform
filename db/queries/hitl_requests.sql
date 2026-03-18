-- name: CreateHITLRequest :one
INSERT INTO hitl_requests (
    sop_execution_id, sop_id, tenant_id, step_id, step_name,
    decision, deadline, payload, temporal_workflow_id, temporal_run_id, version
) VALUES (
    $1, $2, $3, $4, $5,
    'PENDING', $6, $7, $8, $9, 1
)
RETURNING *;

-- name: GetHITLRequestByID :one
SELECT * FROM hitl_requests
WHERE request_id = $1 AND tenant_id = $2;

-- name: ListPendingHITLRequests :many
SELECT * FROM hitl_requests
WHERE tenant_id = $1
  AND decision = 'PENDING'
ORDER BY deadline ASC
LIMIT $2 OFFSET $3;

-- name: CountPendingHITLRequests :one
SELECT COUNT(*) FROM hitl_requests
WHERE tenant_id = $1
  AND decision = 'PENDING';

-- name: ListOverdueHITLRequests :many
SELECT * FROM hitl_requests
WHERE decision = 'PENDING'
  AND deadline < NOW()
ORDER BY deadline ASC
LIMIT $1;

-- name: UpdateHITLDecision :one
UPDATE hitl_requests
SET decision = $2,
    decided_by = $3,
    decision_reason = $4,
    decided_at = NOW(),
    version = version + 1
WHERE request_id = $1
  AND decision = 'PENDING'
  AND version = $5
RETURNING *;

-- name: ListHITLRequestsByExecution :many
SELECT * FROM hitl_requests
WHERE sop_execution_id = $1
ORDER BY created_at ASC;
