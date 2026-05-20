-- SOP execution status enum
CREATE TYPE sop_execution_status AS ENUM (
    'PENDING',
    'RUNNING',
    'WAITING_HITL',
    'COMPLETED',
    'FAILED',
    'CANCELED',
    'ESCALATED'
);

-- SOP industry enum
CREATE TYPE sop_industry AS ENUM (
    'FINANCIAL_SERVICES',
    'INSURANCE',
    'HEALTHCARE',
    'HOSPITAL_OPS',
    'LIFE_SCIENCES',
    'MANUFACTURING'
);

-- SOP executions: runtime instances of SOP workflows
CREATE TABLE sop_executions (
    sop_execution_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sop_id               VARCHAR(20) NOT NULL,
    tenant_id            UUID NOT NULL,
    industry             sop_industry NOT NULL,
    current_step         VARCHAR(100) NOT NULL DEFAULT 'intake',
    status               sop_execution_status NOT NULL DEFAULT 'PENDING',
    input_payload        JSONB NOT NULL DEFAULT '{}',
    output_payload       JSONB,
    temporal_workflow_id VARCHAR(255),
    temporal_run_id      VARCHAR(255),
    started_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version              INTEGER NOT NULL DEFAULT 1,

    CONSTRAINT chk_sop_version CHECK (version > 0)
);

-- Indexes for common query patterns
CREATE INDEX idx_sop_exec_tenant_id ON sop_executions (tenant_id);
CREATE INDEX idx_sop_exec_sop_id ON sop_executions (sop_id);
CREATE INDEX idx_sop_exec_status ON sop_executions (status);
CREATE INDEX idx_sop_exec_industry ON sop_executions (industry);
CREATE INDEX idx_sop_exec_tenant_sop ON sop_executions (tenant_id, sop_id);
CREATE INDEX idx_sop_exec_created_at ON sop_executions (created_at DESC);
CREATE INDEX idx_sop_exec_temporal_wf ON sop_executions (temporal_workflow_id) WHERE temporal_workflow_id IS NOT NULL;

-- Auto-update updated_at
CREATE TRIGGER trg_sop_executions_updated_at
    BEFORE UPDATE ON sop_executions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
