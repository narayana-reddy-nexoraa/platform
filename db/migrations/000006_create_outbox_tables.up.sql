-- Outbox events for reliable event delivery
CREATE TABLE outbox_events (
    event_id        UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type  VARCHAR(100)    NOT NULL,
    aggregate_id    UUID            NOT NULL,
    event_type      VARCHAR(100)    NOT NULL,
    payload         JSONB           NOT NULL,
    metadata        JSONB,
    sequence_number BIGSERIAL       NOT NULL,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    sent_at         TIMESTAMPTZ,
    sent            BOOLEAN         NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_outbox_unsent ON outbox_events (sequence_number ASC) WHERE sent = FALSE;
CREATE INDEX idx_outbox_cleanup ON outbox_events (sent_at) WHERE sent = TRUE;
CREATE INDEX idx_outbox_aggregate ON outbox_events (aggregate_type, aggregate_id, sequence_number);

-- Consumer deduplication tracking
CREATE TABLE processed_events (
    event_id       UUID            NOT NULL,
    consumer_group VARCHAR(100)    NOT NULL,
    processed_at   TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, consumer_group)
);

-- Worker processing audit log (for double-execution detection)
CREATE TABLE processing_log (
    log_id         UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id   UUID            NOT NULL,
    worker_id      VARCHAR(255)    NOT NULL,
    action         VARCHAR(50)     NOT NULL,
    attempt_number INTEGER         NOT NULL,
    created_at     TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_processing_log_execution ON processing_log (execution_id, created_at);
