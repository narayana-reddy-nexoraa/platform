CREATE TABLE consumer_offsets (
    consumer_group      VARCHAR(100)    PRIMARY KEY,
    last_processed_seq  BIGINT          NOT NULL DEFAULT 0,
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
