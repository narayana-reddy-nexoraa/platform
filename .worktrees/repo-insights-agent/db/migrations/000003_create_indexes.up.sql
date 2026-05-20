-- Workers finding claimable jobs: status=CREATED or FAILED with no/expired lock
CREATE INDEX idx_executions_claimable
    ON executions (tenant_id, status, lock_expires_at)
    WHERE status IN ('CREATED', 'FAILED');

-- Reaper detecting expired leases from dead workers
CREATE INDEX idx_executions_expired_leases
    ON executions (lock_expires_at)
    WHERE locked_by IS NOT NULL;

-- API list/filter queries per tenant
CREATE INDEX idx_executions_tenant_status
    ON executions (tenant_id, status, created_at DESC);
