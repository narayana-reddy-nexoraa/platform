-- Agent step type enum
CREATE TYPE agent_step_type AS ENUM (
    'INTAKE',
    'DATA_RETRIEVAL',
    'CLASSIFICATION',
    'DECISIONING',
    'EXECUTION',
    'AUDIT'
);

-- Audit trail: immutable record of every agent action in SOP execution
CREATE TABLE audit_trail (
    audit_id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sop_execution_id     UUID NOT NULL REFERENCES sop_executions(sop_execution_id),
    sop_id               VARCHAR(20) NOT NULL,
    tenant_id            UUID NOT NULL,
    step_id              VARCHAR(100) NOT NULL,
    agent_type           agent_step_type NOT NULL,
    action               VARCHAR(255) NOT NULL,
    input_hash           VARCHAR(64) NOT NULL,
    output_hash          VARCHAR(64) NOT NULL,
    model_used           VARCHAR(100),
    latency_ms           BIGINT NOT NULL DEFAULT 0,
    tokens_used          INTEGER,
    compliance_tags      TEXT[] NOT NULL DEFAULT '{}',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for audit query patterns
CREATE INDEX idx_audit_sop_exec ON audit_trail (sop_execution_id);
CREATE INDEX idx_audit_tenant ON audit_trail (tenant_id);
CREATE INDEX idx_audit_sop_id ON audit_trail (sop_id);
CREATE INDEX idx_audit_created_at ON audit_trail (created_at DESC);
CREATE INDEX idx_audit_tenant_sop ON audit_trail (tenant_id, sop_id, created_at DESC);
CREATE INDEX idx_audit_compliance ON audit_trail USING GIN (compliance_tags);

-- NOTE: No UPDATE trigger on audit_trail — records are immutable (INSERT only, no UPDATE/DELETE in application code)
