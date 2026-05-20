DROP INDEX IF EXISTS idx_dlq_event_id;
CREATE UNIQUE INDEX idx_dlq_event_id ON dead_letter_events (event_id, consumer_group);
