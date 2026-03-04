CREATE INDEX idx_executions_retry_after ON executions (retry_after)
WHERE retry_after IS NOT NULL AND status IN ('CREATED', 'FAILED');
