CREATE TABLE dead_letter_events (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id        UUID            NOT NULL,
    consumer_group  VARCHAR(100)    NOT NULL,
    event_type      VARCHAR(100)    NOT NULL,
    aggregate_type  VARCHAR(100)    NOT NULL,
    aggregate_id    UUID            NOT NULL,
    payload         JSONB           NOT NULL,
    metadata        JSONB,
    error_message   TEXT            NOT NULL,
    attempt_count   INTEGER         NOT NULL DEFAULT 1,
    max_attempts    INTEGER         NOT NULL DEFAULT 3,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    last_failed_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dlq_consumer_group ON dead_letter_events (consumer_group, created_at DESC);
CREATE INDEX idx_dlq_event_id ON dead_letter_events (event_id, consumer_group);
