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
