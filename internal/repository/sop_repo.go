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
	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
	"github.com/narayana-platform/execution-engine/internal/repository/db"
)

// SOPRepository defines the contract for SOP execution persistence.
type SOPRepository interface {
	Create(ctx context.Context, sopID string, tenantID uuid.UUID, industry string, payload json.RawMessage, temporalWorkflowID, temporalRunID string) (*sopdomain.SOPExecution, error)
	GetByID(ctx context.Context, executionID, tenantID uuid.UUID) (*sopdomain.SOPExecution, error)
	List(ctx context.Context, tenantID uuid.UUID, sopID, status, industry *string, limit, offset int32) ([]sopdomain.SOPExecution, int64, error)
	UpdateStatus(ctx context.Context, executionID uuid.UUID, status sopdomain.SOPExecutionStatus, currentStep string, version int32) (*sopdomain.SOPExecution, error)
	UpdateTemporalRunID(ctx context.Context, executionID uuid.UUID, runID string) error
	Complete(ctx context.Context, executionID uuid.UUID, outputPayload json.RawMessage, version int32) (*sopdomain.SOPExecution, error)
	Fail(ctx context.Context, executionID uuid.UUID, version int32) (*sopdomain.SOPExecution, error)
}

// PostgresSOPRepository implements SOPRepository using PostgreSQL via sqlc.
type PostgresSOPRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewPostgresSOPRepository creates a new SOP repository.
func NewPostgresSOPRepository(pool *pgxpool.Pool) *PostgresSOPRepository {
	return &PostgresSOPRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *PostgresSOPRepository) Create(ctx context.Context, sopID string, tenantID uuid.UUID, industry string, payload json.RawMessage, temporalWorkflowID, temporalRunID string) (*sopdomain.SOPExecution, error) {
	row, err := r.queries.CreateSOPExecution(ctx, db.CreateSOPExecutionParams{
		SopID:              sopID,
		TenantID:           tenantID,
		Industry:           db.SopIndustry(industry),
		InputPayload:       payload,
		TemporalWorkflowID: pgtype.Text{String: temporalWorkflowID, Valid: temporalWorkflowID != ""},
		TemporalRunID:      pgtype.Text{String: temporalRunID, Valid: temporalRunID != ""},
	})
	if err != nil {
		return nil, err
	}
	return mapSOPExecution(row), nil
}

func (r *PostgresSOPRepository) GetByID(ctx context.Context, executionID, tenantID uuid.UUID) (*sopdomain.SOPExecution, error) {
	row, err := r.queries.GetSOPExecutionByID(ctx, db.GetSOPExecutionByIDParams{
		SopExecutionID: executionID,
		TenantID:       tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrNotFound{Entity: "sop_execution", ID: executionID.String()}
		}
		return nil, err
	}
	return mapSOPExecution(row), nil
}

func (r *PostgresSOPRepository) List(ctx context.Context, tenantID uuid.UUID, sopID, status, industry *string, limit, offset int32) ([]sopdomain.SOPExecution, int64, error) {
	listParams := db.ListSOPExecutionsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	}
	countParams := db.CountSOPExecutionsParams{
		TenantID: tenantID,
	}

	if sopID != nil {
		listParams.SopID = pgtype.Text{String: *sopID, Valid: true}
		countParams.SopID = pgtype.Text{String: *sopID, Valid: true}
	}
	if status != nil {
		listParams.Status = db.NullSopExecutionStatus{SopExecutionStatus: db.SopExecutionStatus(*status), Valid: true}
		countParams.Status = db.NullSopExecutionStatus{SopExecutionStatus: db.SopExecutionStatus(*status), Valid: true}
	}
	if industry != nil {
		listParams.Industry = db.NullSopIndustry{SopIndustry: db.SopIndustry(*industry), Valid: true}
		countParams.Industry = db.NullSopIndustry{SopIndustry: db.SopIndustry(*industry), Valid: true}
	}

	rows, err := r.queries.ListSOPExecutions(ctx, listParams)
	if err != nil {
		return nil, 0, err
	}

	count, err := r.queries.CountSOPExecutions(ctx, countParams)
	if err != nil {
		return nil, 0, err
	}

	executions := make([]sopdomain.SOPExecution, len(rows))
	for i, row := range rows {
		executions[i] = *mapSOPExecution(row)
	}
	return executions, count, nil
}

func (r *PostgresSOPRepository) UpdateStatus(ctx context.Context, executionID uuid.UUID, status sopdomain.SOPExecutionStatus, currentStep string, version int32) (*sopdomain.SOPExecution, error) {
	row, err := r.queries.UpdateSOPExecutionStatus(ctx, db.UpdateSOPExecutionStatusParams{
		SopExecutionID: executionID,
		Status:         db.SopExecutionStatus(status),
		CurrentStep:    currentStep,
		Version:        version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	return mapSOPExecution(row), nil
}

func (r *PostgresSOPRepository) Complete(ctx context.Context, executionID uuid.UUID, outputPayload json.RawMessage, version int32) (*sopdomain.SOPExecution, error) {
	row, err := r.queries.CompleteSOPExecution(ctx, db.CompleteSOPExecutionParams{
		SopExecutionID: executionID,
		OutputPayload:  outputPayload,
		Version:        version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	return mapSOPExecution(row), nil
}

func (r *PostgresSOPRepository) Fail(ctx context.Context, executionID uuid.UUID, version int32) (*sopdomain.SOPExecution, error) {
	row, err := r.queries.FailSOPExecution(ctx, db.FailSOPExecutionParams{
		SopExecutionID: executionID,
		Version:        version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: executionID.String()}
		}
		return nil, err
	}
	return mapSOPExecution(row), nil
}

func (r *PostgresSOPRepository) UpdateTemporalRunID(ctx context.Context, executionID uuid.UUID, runID string) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE sop_executions SET temporal_run_id = $1 WHERE sop_execution_id = $2",
		runID, executionID)
	return err
}

// mapSOPExecution converts sqlc-generated type to domain type.
func mapSOPExecution(row db.SopExecution) *sopdomain.SOPExecution {
	exec := &sopdomain.SOPExecution{
		SOPExecutionID: row.SopExecutionID,
		SOPID:          row.SopID,
		TenantID:       row.TenantID,
		Industry:        sopdomain.Industry(row.Industry),
		CurrentStep:    row.CurrentStep,
		Status:         sopdomain.SOPExecutionStatus(row.Status),
		InputPayload:   row.InputPayload,
		OutputPayload:  row.OutputPayload,
		Version:        row.Version,
	}
	if row.TemporalWorkflowID.Valid {
		exec.TemporalWorkflowID = row.TemporalWorkflowID.String
	}
	if row.TemporalRunID.Valid {
		exec.TemporalRunID = row.TemporalRunID.String
	}
	if row.StartedAt.Valid {
		exec.StartedAt = row.StartedAt.Time
	}
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time
		exec.CompletedAt = &t
	}
	if row.CreatedAt.Valid {
		exec.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		exec.UpdatedAt = row.UpdatedAt.Time
	}
	return exec
}

// pgtimeOrNow returns the pgtype time if valid, otherwise now.
func pgtimeOrNow(t pgtype.Timestamptz) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Now()
}
