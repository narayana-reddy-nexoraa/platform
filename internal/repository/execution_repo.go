package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository/db"
)

// ExecutionRepository defines the contract for execution persistence.
type ExecutionRepository interface {
	CreateIdempotent(ctx context.Context, tenantID uuid.UUID, idempotencyKey string, maxAttempts int32, payload json.RawMessage, payloadHash string) (*domain.Execution, bool, error)
	GetByID(ctx context.Context, executionID, tenantID uuid.UUID) (*domain.Execution, error)
	List(ctx context.Context, tenantID uuid.UUID, status *domain.ExecutionStatus, limit, offset int32) ([]domain.Execution, int64, error)
	FindClaimable(ctx context.Context, limit int32) ([]domain.Execution, error)
	Claim(ctx context.Context, executionID uuid.UUID, workerID string, leaseDuration int32, version int32) (*domain.Execution, error)
	UpdateStatus(ctx context.Context, executionID uuid.UUID, status domain.ExecutionStatus, version int32) (*domain.Execution, error)
	Complete(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error)
	Fail(ctx context.Context, executionID uuid.UUID, errorCode, errorMessage string, version int32) (*domain.Execution, error)
	SendHeartbeat(ctx context.Context, executionID uuid.UUID, leaseDuration int32, workerID string) (*domain.Execution, error)
	FindExpiredLeases(ctx context.Context, limit int32) ([]domain.Execution, error)
	Reclaim(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error)
	Retry(ctx context.Context, executionID uuid.UUID, errorCode, errorMessage string, delayMs int64, version int32) (*domain.Execution, error)
	InsertTransition(ctx context.Context, executionID uuid.UUID, fromStatus, toStatus domain.ExecutionStatus, triggeredBy, reason string) error

	// Transactional outbox methods — execute mutation + transition + outbox event in one TX.
	CompleteWithOutbox(ctx context.Context, executionID uuid.UUID, workerID string, version int32) (*domain.Execution, error)
	FailWithOutbox(ctx context.Context, executionID uuid.UUID, workerID string, errorCode, errorMessage string, version int32) (*domain.Execution, error)
	RetryWithOutbox(ctx context.Context, executionID uuid.UUID, workerID string, errorCode, errorMessage string, delayMs int64, version int32) (*domain.Execution, error)
	ReclaimWithOutbox(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error)
	ClaimWithOutbox(ctx context.Context, executionID uuid.UUID, workerID string, leaseDuration int32, version int32) (*domain.Execution, error)

	// Outbox operations
	FetchUnsentEvents(ctx context.Context, limit int32) ([]domain.OutboxEvent, error)
	MarkEventsSent(ctx context.Context, eventIDs []uuid.UUID) error
	CleanupOldEvents(ctx context.Context, olderThan time.Time) error
	CountUnsentEvents(ctx context.Context) (int64, error)

	// Processing
	InsertProcessingLog(ctx context.Context, executionID uuid.UUID, workerID, action string, attemptNumber int32) error
	InsertProcessedEvent(ctx context.Context, eventID uuid.UUID, consumerGroup string) (bool, error)

	// Dead Letter Queue
	InsertDLQEvent(ctx context.Context, evt domain.OutboxEvent, consumerGroup string, errMsg string) error
	ListDLQEvents(ctx context.Context, consumerGroup string, limit, offset int32) ([]domain.DeadLetterEvent, error)
	DeleteDLQEvent(ctx context.Context, id uuid.UUID) error
	CountDLQEvents(ctx context.Context, consumerGroup string) (int64, error)

	// Consumer offset tracking
	GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error)
	UpsertConsumerOffset(ctx context.Context, consumerGroup string, lastProcessedSeq int64) error
}

// PostgresExecutionRepository implements ExecutionRepository using PostgreSQL via sqlc.
type PostgresExecutionRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewPostgresExecutionRepository creates a new repository backed by PostgreSQL.
func NewPostgresExecutionRepository(pool *pgxpool.Pool) *PostgresExecutionRepository {
	return &PostgresExecutionRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

// CreateIdempotent attempts to insert a new execution. Returns (exec, isNew, error).
// If the idempotency key already exists with the same payload hash, returns the existing row.
// If the key exists with a different payload hash, returns ErrIdempotencyConflict.
func (r *PostgresExecutionRepository) CreateIdempotent(
	ctx context.Context,
	tenantID uuid.UUID,
	idempotencyKey string,
	maxAttempts int32,
	payload json.RawMessage,
	payloadHash string,
) (*domain.Execution, bool, error) {
	// Attempt INSERT ... ON CONFLICT DO NOTHING
	row, err := r.queries.CreateExecution(ctx, db.CreateExecutionParams{
		TenantID:       tenantID,
		IdempotencyKey: idempotencyKey,
		MaxAttempts:    maxAttempts,
		Payload:        []byte(payload),
		PayloadHash:    payloadHash,
	})

	if err == nil {
		// New row was inserted
		exec := toDomain(row)
		return &exec, true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}

	// ON CONFLICT hit — key already exists. Fetch existing row.
	existing, err := r.queries.GetExecutionByTenantAndIdempotencyKey(ctx, db.GetExecutionByTenantAndIdempotencyKeyParams{
		TenantID:       tenantID,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return nil, false, err
	}

	exec := toDomain(existing)

	// Compare payload hash for conflict detection
	if existing.PayloadHash != payloadHash {
		return nil, false, &domain.ErrIdempotencyConflict{IdempotencyKey: idempotencyKey}
	}

	return &exec, false, nil
}

func (r *PostgresExecutionRepository) GetByID(ctx context.Context, executionID, tenantID uuid.UUID) (*domain.Execution, error) {
	row, err := r.queries.GetExecutionByID(ctx, db.GetExecutionByIDParams{
		ExecutionID: executionID,
		TenantID:    tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrNotFound{Entity: "execution", ID: executionID.String()}
		}
		return nil, err
	}
	exec := toDomain(row)
	return &exec, nil
}

func (r *PostgresExecutionRepository) List(ctx context.Context, tenantID uuid.UUID, status *domain.ExecutionStatus, limit, offset int32) ([]domain.Execution, int64, error) {
	// Build nullable status filter
	var nullStatus db.NullExecutionStatus
	if status != nil {
		nullStatus = db.NullExecutionStatus{
			ExecutionStatus: db.ExecutionStatus(*status),
			Valid:           true,
		}
	}

	rows, err := r.queries.ListExecutions(ctx, db.ListExecutionsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
		Status:   nullStatus,
	})
	if err != nil {
		return nil, 0, err
	}

	count, err := r.queries.CountExecutions(ctx, db.CountExecutionsParams{
		TenantID: tenantID,
		Status:   nullStatus,
	})
	if err != nil {
		return nil, 0, err
	}

	executions := make([]domain.Execution, len(rows))
	for i, row := range rows {
		executions[i] = toDomain(row)
	}

	return executions, count, nil
}

func (r *PostgresExecutionRepository) FindClaimable(ctx context.Context, limit int32) ([]domain.Execution, error) {
	rows, err := r.queries.FindClaimableExecutions(ctx, limit)
	if err != nil {
		return nil, err
	}

	executions := make([]domain.Execution, len(rows))
	for i, row := range rows {
		executions[i] = toDomain(row)
	}
	return executions, nil
}

func (r *PostgresExecutionRepository) Claim(ctx context.Context, executionID uuid.UUID, workerID string, leaseDuration int32, version int32) (*domain.Execution, error) {
	row, err := r.queries.ClaimExecution(ctx, db.ClaimExecutionParams{
		ExecutionID: executionID,
		LockedBy:    pgtype.Text{String: workerID, Valid: true},
		Column3:     leaseDuration,
		Version:     version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	exec := toDomain(row)
	return &exec, nil
}

func (r *PostgresExecutionRepository) UpdateStatus(ctx context.Context, executionID uuid.UUID, status domain.ExecutionStatus, version int32) (*domain.Execution, error) {
	row, err := r.queries.UpdateExecutionStatus(ctx, db.UpdateExecutionStatusParams{
		ExecutionID: executionID,
		Status:      db.ExecutionStatus(status),
		Version:     version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	exec := toDomain(row)
	return &exec, nil
}

func (r *PostgresExecutionRepository) Complete(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error) {
	row, err := r.queries.CompleteExecution(ctx, db.CompleteExecutionParams{
		ExecutionID: executionID,
		Version:     version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	exec := toDomain(row)
	return &exec, nil
}

func (r *PostgresExecutionRepository) Fail(ctx context.Context, executionID uuid.UUID, errorCode, errorMessage string, version int32) (*domain.Execution, error) {
	row, err := r.queries.FailExecution(ctx, db.FailExecutionParams{
		ExecutionID:  executionID,
		ErrorCode:    pgtype.Text{String: errorCode, Valid: true},
		ErrorMessage: pgtype.Text{String: errorMessage, Valid: true},
		Version:      version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	exec := toDomain(row)
	return &exec, nil
}

func (r *PostgresExecutionRepository) SendHeartbeat(ctx context.Context, executionID uuid.UUID, leaseDuration int32, workerID string) (*domain.Execution, error) {
	row, err := r.queries.SendHeartbeat(ctx, db.SendHeartbeatParams{
		ExecutionID: executionID,
		Column2:     leaseDuration,
		LockedBy:    pgtype.Text{String: workerID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrLostLease{ExecutionID: executionID.String(), WorkerID: workerID}
		}
		return nil, err
	}
	exec := toDomain(row)
	return &exec, nil
}

func (r *PostgresExecutionRepository) FindExpiredLeases(ctx context.Context, limit int32) ([]domain.Execution, error) {
	rows, err := r.queries.FindExpiredLeases(ctx, limit)
	if err != nil {
		return nil, err
	}
	executions := make([]domain.Execution, len(rows))
	for i, row := range rows {
		executions[i] = toDomain(row)
	}
	return executions, nil
}

func (r *PostgresExecutionRepository) Reclaim(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error) {
	row, err := r.queries.ReclaimExecution(ctx, db.ReclaimExecutionParams{
		ExecutionID: executionID,
		Version:     version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	exec := toDomain(row)
	return &exec, nil
}

func (r *PostgresExecutionRepository) Retry(ctx context.Context, executionID uuid.UUID, errorCode, errorMessage string, delayMs int64, version int32) (*domain.Execution, error) {
	row, err := r.queries.RetryExecution(ctx, db.RetryExecutionParams{
		ExecutionID:  executionID,
		ErrorCode:    pgtype.Text{String: errorCode, Valid: true},
		ErrorMessage: pgtype.Text{String: errorMessage, Valid: true},
		Column4:      delayMs,
		Version:      version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	exec := toDomain(row)
	return &exec, nil
}

func (r *PostgresExecutionRepository) InsertTransition(ctx context.Context, executionID uuid.UUID, fromStatus, toStatus domain.ExecutionStatus, triggeredBy, reason string) error {
	return r.queries.InsertTransition(ctx, db.InsertTransitionParams{
		ExecutionID: executionID,
		FromStatus:  db.ExecutionStatus(fromStatus),
		ToStatus:    db.ExecutionStatus(toStatus),
		TriggeredBy: triggeredBy,
		Reason:      pgtype.Text{String: reason, Valid: reason != ""},
	})
}

// ---------------------------------------------------------------------------
// Transactional WithOutbox methods
// ---------------------------------------------------------------------------

// CompleteWithOutbox atomically completes an execution (RUNNING → SUCCEEDED),
// inserts a transition record, and publishes an outbox event.
func (r *PostgresExecutionRepository) CompleteWithOutbox(ctx context.Context, executionID uuid.UUID, workerID string, version int32) (*domain.Execution, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)

	row, err := qtx.CompleteExecution(ctx, db.CompleteExecutionParams{
		ExecutionID: executionID,
		Version:     version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}

	exec := toDomain(row)

	if err := qtx.InsertTransition(ctx, db.InsertTransitionParams{
		ExecutionID: executionID,
		FromStatus:  db.ExecutionStatusRUNNING,
		ToStatus:    db.ExecutionStatusSUCCEEDED,
		TriggeredBy: workerID,
		Reason:      pgtype.Text{String: "execution completed successfully", Valid: true},
	}); err != nil {
		return nil, err
	}

	payload, metadata, err := domain.NewExecutionEvent(domain.EventExecutionSucceeded, &exec, "RUNNING", workerID)
	if err != nil {
		return nil, err
	}

	if err := qtx.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		AggregateType: "execution",
		AggregateID:   executionID,
		EventType:     domain.EventExecutionSucceeded,
		Payload:       payload,
		Metadata:      metadata,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &exec, nil
}

// FailWithOutbox atomically fails an execution (RUNNING → FAILED),
// inserts a transition record, and publishes an outbox event.
func (r *PostgresExecutionRepository) FailWithOutbox(ctx context.Context, executionID uuid.UUID, workerID string, errorCode, errorMessage string, version int32) (*domain.Execution, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)

	row, err := qtx.FailExecution(ctx, db.FailExecutionParams{
		ExecutionID:  executionID,
		ErrorCode:    pgtype.Text{String: errorCode, Valid: true},
		ErrorMessage: pgtype.Text{String: errorMessage, Valid: true},
		Version:      version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}

	exec := toDomain(row)

	if err := qtx.InsertTransition(ctx, db.InsertTransitionParams{
		ExecutionID: executionID,
		FromStatus:  db.ExecutionStatusRUNNING,
		ToStatus:    db.ExecutionStatusFAILED,
		TriggeredBy: workerID,
		Reason:      pgtype.Text{String: errorMessage, Valid: true},
	}); err != nil {
		return nil, err
	}

	payload, metadata, err := domain.NewExecutionEvent(domain.EventExecutionFailed, &exec, "RUNNING", workerID)
	if err != nil {
		return nil, err
	}

	if err := qtx.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		AggregateType: "execution",
		AggregateID:   executionID,
		EventType:     domain.EventExecutionFailed,
		Payload:       payload,
		Metadata:      metadata,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &exec, nil
}

// RetryWithOutbox atomically schedules a retry (RUNNING → CREATED with retry_after),
// inserts a transition record, and publishes an outbox event.
func (r *PostgresExecutionRepository) RetryWithOutbox(ctx context.Context, executionID uuid.UUID, workerID string, errorCode, errorMessage string, delayMs int64, version int32) (*domain.Execution, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)

	row, err := qtx.RetryExecution(ctx, db.RetryExecutionParams{
		ExecutionID:  executionID,
		ErrorCode:    pgtype.Text{String: errorCode, Valid: true},
		ErrorMessage: pgtype.Text{String: errorMessage, Valid: true},
		Column4:      delayMs,
		Version:      version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}

	exec := toDomain(row)

	if err := qtx.InsertTransition(ctx, db.InsertTransitionParams{
		ExecutionID: executionID,
		FromStatus:  db.ExecutionStatusRUNNING,
		ToStatus:    db.ExecutionStatusCREATED,
		TriggeredBy: workerID,
		Reason:      pgtype.Text{String: errorMessage, Valid: true},
	}); err != nil {
		return nil, err
	}

	payload, metadata, err := domain.NewExecutionEvent(domain.EventExecutionRetryScheduled, &exec, "RUNNING", workerID)
	if err != nil {
		return nil, err
	}

	if err := qtx.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		AggregateType: "execution",
		AggregateID:   executionID,
		EventType:     domain.EventExecutionRetryScheduled,
		Payload:       payload,
		Metadata:      metadata,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &exec, nil
}

// ReclaimWithOutbox atomically reclaims an execution (CLAIMED → CREATED),
// inserts a transition record, and publishes an outbox event.
func (r *PostgresExecutionRepository) ReclaimWithOutbox(ctx context.Context, executionID uuid.UUID, version int32) (*domain.Execution, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)

	row, err := qtx.ReclaimExecution(ctx, db.ReclaimExecutionParams{
		ExecutionID: executionID,
		Version:     version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}

	exec := toDomain(row)

	if err := qtx.InsertTransition(ctx, db.InsertTransitionParams{
		ExecutionID: executionID,
		FromStatus:  db.ExecutionStatusCLAIMED,
		ToStatus:    db.ExecutionStatusCREATED,
		TriggeredBy: "reaper",
		Reason:      pgtype.Text{String: "lease expired, reclaiming execution", Valid: true},
	}); err != nil {
		return nil, err
	}

	payload, metadata, err := domain.NewExecutionEvent(domain.EventExecutionReclaimed, &exec, "CLAIMED", "reaper")
	if err != nil {
		return nil, err
	}

	if err := qtx.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		AggregateType: "execution",
		AggregateID:   executionID,
		EventType:     domain.EventExecutionReclaimed,
		Payload:       payload,
		Metadata:      metadata,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &exec, nil
}

// ClaimWithOutbox atomically claims an execution (CREATED → CLAIMED),
// inserts a transition record, and publishes an outbox event.
func (r *PostgresExecutionRepository) ClaimWithOutbox(ctx context.Context, executionID uuid.UUID, workerID string, leaseDuration int32, version int32) (*domain.Execution, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)

	row, err := qtx.ClaimExecution(ctx, db.ClaimExecutionParams{
		ExecutionID: executionID,
		LockedBy:    pgtype.Text{String: workerID, Valid: true},
		Column3:     leaseDuration,
		Version:     version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}

	exec := toDomain(row)

	if err := qtx.InsertTransition(ctx, db.InsertTransitionParams{
		ExecutionID: executionID,
		FromStatus:  db.ExecutionStatusCREATED,
		ToStatus:    db.ExecutionStatusCLAIMED,
		TriggeredBy: workerID,
		Reason:      pgtype.Text{String: "execution claimed by worker", Valid: true},
	}); err != nil {
		return nil, err
	}

	payload, metadata, err := domain.NewExecutionEvent(domain.EventExecutionClaimed, &exec, "CREATED", workerID)
	if err != nil {
		return nil, err
	}

	if err := qtx.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{
		AggregateType: "execution",
		AggregateID:   executionID,
		EventType:     domain.EventExecutionClaimed,
		Payload:       payload,
		Metadata:      metadata,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &exec, nil
}

// ---------------------------------------------------------------------------
// Outbox operations
// ---------------------------------------------------------------------------

// FetchUnsentEvents retrieves unsent outbox events within a transaction
// (required because the query uses FOR UPDATE SKIP LOCKED).
func (r *PostgresExecutionRepository) FetchUnsentEvents(ctx context.Context, limit int32) ([]domain.OutboxEvent, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := db.New(tx)
	rows, err := qtx.FetchUnsentEvents(ctx, limit)
	if err != nil {
		return nil, err
	}

	events := make([]domain.OutboxEvent, len(rows))
	for i, row := range rows {
		events[i] = toOutboxDomain(row)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return events, nil
}

// MarkEventsSent marks the given outbox events as sent.
func (r *PostgresExecutionRepository) MarkEventsSent(ctx context.Context, eventIDs []uuid.UUID) error {
	return r.queries.MarkEventsSent(ctx, eventIDs)
}

// CleanupOldEvents deletes sent outbox events older than the given time.
func (r *PostgresExecutionRepository) CleanupOldEvents(ctx context.Context, olderThan time.Time) error {
	return r.queries.CleanupOldEvents(ctx, pgtype.Timestamptz{Time: olderThan, Valid: true})
}

// CountUnsentEvents returns the number of unsent outbox events.
func (r *PostgresExecutionRepository) CountUnsentEvents(ctx context.Context) (int64, error) {
	return r.queries.CountUnsentEvents(ctx)
}

// ---------------------------------------------------------------------------
// Processing operations
// ---------------------------------------------------------------------------

// InsertProcessingLog records an action in the processing log.
func (r *PostgresExecutionRepository) InsertProcessingLog(ctx context.Context, executionID uuid.UUID, workerID, action string, attemptNumber int32) error {
	return r.queries.InsertProcessingLog(ctx, db.InsertProcessingLogParams{
		ExecutionID:   executionID,
		WorkerID:      workerID,
		Action:        action,
		AttemptNumber: attemptNumber,
	})
}

// InsertProcessedEvent records an event as processed for idempotent consumption.
// Returns (true, nil) if newly inserted, (false, nil) if already processed (duplicate).
func (r *PostgresExecutionRepository) InsertProcessedEvent(ctx context.Context, eventID uuid.UUID, consumerGroup string) (bool, error) {
	_, err := r.queries.InsertProcessedEvent(ctx, db.InsertProcessedEventParams{
		EventID:       eventID,
		ConsumerGroup: consumerGroup,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // duplicate — already processed
		}
		return false, err
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Dead Letter Queue operations
// ---------------------------------------------------------------------------

// InsertDLQEvent inserts a failed event into the dead letter queue.
func (r *PostgresExecutionRepository) InsertDLQEvent(ctx context.Context, evt domain.OutboxEvent, consumerGroup string, errMsg string) error {
	return r.queries.InsertDLQEvent(ctx, db.InsertDLQEventParams{
		EventID:       evt.EventID,
		ConsumerGroup: consumerGroup,
		EventType:     evt.EventType,
		AggregateType: evt.AggregateType,
		AggregateID:   evt.AggregateID,
		Payload:       []byte(evt.Payload),
		Metadata:      []byte(evt.Metadata),
		ErrorMessage:  errMsg,
		AttemptCount:  1,
		MaxAttempts:   3,
	})
}

// ListDLQEvents retrieves dead letter events for a consumer group with pagination.
func (r *PostgresExecutionRepository) ListDLQEvents(ctx context.Context, consumerGroup string, limit, offset int32) ([]domain.DeadLetterEvent, error) {
	rows, err := r.queries.ListDLQEvents(ctx, db.ListDLQEventsParams{
		ConsumerGroup: consumerGroup,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return nil, err
	}

	events := make([]domain.DeadLetterEvent, len(rows))
	for i, row := range rows {
		events[i] = toDLQDomain(row)
	}
	return events, nil
}

// DeleteDLQEvent removes a dead letter event by ID.
func (r *PostgresExecutionRepository) DeleteDLQEvent(ctx context.Context, id uuid.UUID) error {
	return r.queries.DeleteDLQEvent(ctx, id)
}

// CountDLQEvents returns the number of dead letter events for a consumer group.
func (r *PostgresExecutionRepository) CountDLQEvents(ctx context.Context, consumerGroup string) (int64, error) {
	return r.queries.CountDLQEvents(ctx, consumerGroup)
}

// ---------------------------------------------------------------------------
// Consumer offset tracking
// ---------------------------------------------------------------------------

// GetConsumerOffset returns the last processed sequence number for a consumer group.
// Returns 0 if no offset has been recorded yet.
func (r *PostgresExecutionRepository) GetConsumerOffset(ctx context.Context, consumerGroup string) (int64, error) {
	row, err := r.queries.GetConsumerOffset(ctx, consumerGroup)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return row.LastProcessedSeq, nil
}

// UpsertConsumerOffset updates the last processed sequence number for a consumer group.
// Uses GREATEST to ensure the offset only moves forward.
func (r *PostgresExecutionRepository) UpsertConsumerOffset(ctx context.Context, consumerGroup string, lastProcessedSeq int64) error {
	return r.queries.UpsertConsumerOffset(ctx, db.UpsertConsumerOffsetParams{
		ConsumerGroup:    consumerGroup,
		LastProcessedSeq: lastProcessedSeq,
	})
}

// ---------------------------------------------------------------------------
// Domain conversion helpers
// ---------------------------------------------------------------------------

// toOutboxDomain converts a sqlc-generated db.OutboxEvent to a domain.OutboxEvent.
func toOutboxDomain(row db.OutboxEvent) domain.OutboxEvent {
	return domain.OutboxEvent{
		EventID:        row.EventID,
		AggregateType:  row.AggregateType,
		AggregateID:    row.AggregateID,
		EventType:      row.EventType,
		Payload:        json.RawMessage(row.Payload),
		Metadata:       json.RawMessage(row.Metadata),
		SequenceNumber: row.SequenceNumber,
		CreatedAt:      pgTimestamptzToTime(row.CreatedAt),
		Sent:           row.Sent,
		SentAt:         pgTimestamptzToTimePtr(row.SentAt),
	}
}

// toDLQDomain converts a sqlc-generated db.DeadLetterEvent to a domain.DeadLetterEvent.
func toDLQDomain(row db.DeadLetterEvent) domain.DeadLetterEvent {
	return domain.DeadLetterEvent{
		ID:            row.ID,
		EventID:       row.EventID,
		ConsumerGroup: row.ConsumerGroup,
		EventType:     row.EventType,
		AggregateType: row.AggregateType,
		AggregateID:   row.AggregateID,
		Payload:       json.RawMessage(row.Payload),
		Metadata:      json.RawMessage(row.Metadata),
		ErrorMessage:  row.ErrorMessage,
		AttemptCount:  row.AttemptCount,
		MaxAttempts:   row.MaxAttempts,
		CreatedAt:     pgTimestamptzToTime(row.CreatedAt),
		LastFailedAt:  pgTimestamptzToTime(row.LastFailedAt),
	}
}

// toDomain converts a sqlc-generated db.Execution to a domain.Execution.
// This is the only place where pgtype types are translated to standard Go types.
func toDomain(row db.Execution) domain.Execution {
	return domain.Execution{
		ExecutionID:     row.ExecutionID,
		TenantID:        row.TenantID,
		IdempotencyKey:  row.IdempotencyKey,
		Status:          domain.ExecutionStatus(row.Status),
		AttemptCount:    row.AttemptCount,
		MaxAttempts:     row.MaxAttempts,
		LockedBy:        pgTextToStringPtr(row.LockedBy),
		LockExpiresAt:   pgTimestamptzToTimePtr(row.LockExpiresAt),
		LastHeartbeatAt: pgTimestamptzToTimePtr(row.LastHeartbeatAt),
		ErrorCode:       pgTextToStringPtr(row.ErrorCode),
		ErrorMessage:    pgTextToStringPtr(row.ErrorMessage),
		Payload:         json.RawMessage(row.Payload),
		PayloadHash:     row.PayloadHash,
		RetryAfter:      pgTimestamptzToTimePtr(row.RetryAfter),
		CreatedAt:       pgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:       pgTimestamptzToTime(row.UpdatedAt),
		Version:         row.Version,
	}
}

func pgTextToStringPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func pgTimestamptzToTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	return &ts.Time
}

func pgTimestamptzToTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}
