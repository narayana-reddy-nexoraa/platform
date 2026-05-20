-- name: InsertAuditEntry :one
INSERT INTO audit_trail (
    sop_execution_id, sop_id, tenant_id, step_id, agent_type,
    action, input_hash, output_hash, model_used, latency_ms,
    tokens_used, compliance_tags
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12
)
RETURNING *;

-- name: ListAuditByExecution :many
SELECT * FROM audit_trail
WHERE sop_execution_id = $1
ORDER BY created_at ASC;

-- name: ListAuditByTenantAndSOP :many
SELECT * FROM audit_trail
WHERE tenant_id = $1
  AND sop_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountAuditByExecution :one
SELECT COUNT(*) FROM audit_trail
WHERE sop_execution_id = $1;
