CREATE TABLE execution_transitions (
    transition_id  UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id   UUID            NOT NULL REFERENCES executions(execution_id),
    from_status    execution_status NOT NULL,
    to_status      execution_status NOT NULL,
    triggered_by   VARCHAR(255)    NOT NULL,
    reason         TEXT,
    metadata       JSONB,
    created_at     TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transitions_execution
    ON execution_transitions (execution_id, created_at);
