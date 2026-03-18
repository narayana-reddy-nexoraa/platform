-- HITL decision enum
CREATE TYPE hitl_decision AS ENUM (
    'PENDING',
    'APPROVED',
    'REJECTED',
    'ESCALATED'
);

-- HITL requests: human-in-the-loop approval queue
CREATE TABLE hitl_requests (
    request_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sop_execution_id     UUID NOT NULL REFERENCES sop_executions(sop_execution_id),
    sop_id               VARCHAR(20) NOT NULL,
    tenant_id            UUID NOT NULL,
    step_id              VARCHAR(100) NOT NULL,
    step_name            VARCHAR(255) NOT NULL,
    decision             hitl_decision NOT NULL DEFAULT 'PENDING',
    decided_by           VARCHAR(255),
    decision_reason      TEXT,
    decided_at           TIMESTAMPTZ,
    deadline             TIMESTAMPTZ NOT NULL,
    payload              JSONB NOT NULL DEFAULT '{}',
    temporal_workflow_id VARCHAR(255) NOT NULL,
    temporal_run_id      VARCHAR(255) NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version              INTEGER NOT NULL DEFAULT 1,

    CONSTRAINT chk_hitl_version CHECK (version > 0)
);

-- Indexes for HITL query patterns
CREATE INDEX idx_hitl_tenant_id ON hitl_requests (tenant_id);
CREATE INDEX idx_hitl_pending ON hitl_requests (tenant_id, decision) WHERE decision = 'PENDING';
CREATE INDEX idx_hitl_sop_exec ON hitl_requests (sop_execution_id);
CREATE INDEX idx_hitl_deadline ON hitl_requests (deadline) WHERE decision = 'PENDING';
CREATE INDEX idx_hitl_created_at ON hitl_requests (created_at DESC);

-- Auto-update updated_at
CREATE TRIGGER trg_hitl_requests_updated_at
    BEFORE UPDATE ON hitl_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
