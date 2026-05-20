CREATE TABLE executions (
    execution_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL,
    idempotency_key  VARCHAR(255) NOT NULL,
    status           execution_status NOT NULL DEFAULT 'CREATED',
    attempt_count    INTEGER NOT NULL DEFAULT 0,
    max_attempts     INTEGER NOT NULL DEFAULT 3,
    locked_by        VARCHAR(255),
    lock_expires_at  TIMESTAMPTZ,
    last_heartbeat_at TIMESTAMPTZ,
    error_code       VARCHAR(100),
    error_message    TEXT,
    payload          JSONB NOT NULL DEFAULT '{}',
    payload_hash     VARCHAR(64) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    version          INTEGER NOT NULL DEFAULT 1,

    CONSTRAINT uq_tenant_idempotency UNIQUE (tenant_id, idempotency_key),
    CONSTRAINT chk_attempt_count CHECK (attempt_count >= 0),
    CONSTRAINT chk_max_attempts CHECK (max_attempts > 0),
    CONSTRAINT chk_version CHECK (version > 0)
);

-- Auto-update updated_at on every UPDATE
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_executions_updated_at
    BEFORE UPDATE ON executions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
