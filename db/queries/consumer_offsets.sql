-- name: GetConsumerOffset :one
SELECT * FROM consumer_offsets WHERE consumer_group = $1;

-- name: UpsertConsumerOffset :exec
INSERT INTO consumer_offsets (consumer_group, last_processed_seq, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (consumer_group) DO UPDATE
SET last_processed_seq = GREATEST(consumer_offsets.last_processed_seq, $2),
    updated_at = NOW();
