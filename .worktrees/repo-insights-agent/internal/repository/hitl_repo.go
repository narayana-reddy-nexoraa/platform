package repository

import (
	"context"
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

// HITLRepository defines the contract for HITL request persistence.
type HITLRepository interface {
	GetByID(ctx context.Context, requestID, tenantID uuid.UUID) (*sopdomain.HITLRequest, error)
	ListPending(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]sopdomain.HITLRequest, int64, error)
	UpdateDecision(ctx context.Context, requestID uuid.UUID, decision sopdomain.HITLDecision, decidedBy, reason string, version int32) (*sopdomain.HITLRequest, error)
	ListByExecution(ctx context.Context, executionID uuid.UUID) ([]sopdomain.HITLRequest, error)
}

// PostgresHITLRepository implements HITLRepository using PostgreSQL via sqlc.
type PostgresHITLRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewPostgresHITLRepository creates a new HITL repository.
func NewPostgresHITLRepository(pool *pgxpool.Pool) *PostgresHITLRepository {
	return &PostgresHITLRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *PostgresHITLRepository) GetByID(ctx context.Context, requestID, tenantID uuid.UUID) (*sopdomain.HITLRequest, error) {
	row, err := r.queries.GetHITLRequestByID(ctx, db.GetHITLRequestByIDParams{
		RequestID: requestID,
		TenantID:  tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrNotFound{Entity: "hitl_request", ID: requestID.String()}
		}
		return nil, err
	}
	return mapHITLRequest(row), nil
}

func (r *PostgresHITLRepository) ListPending(ctx context.Context, tenantID uuid.UUID, limit, offset int32) ([]sopdomain.HITLRequest, int64, error) {
	rows, err := r.queries.ListPendingHITLRequests(ctx, db.ListPendingHITLRequestsParams{
		TenantID: tenantID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, 0, err
	}

	count, err := r.queries.CountPendingHITLRequests(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	requests := make([]sopdomain.HITLRequest, len(rows))
	for i, row := range rows {
		requests[i] = *mapHITLRequest(row)
	}
	return requests, count, nil
}

func (r *PostgresHITLRepository) UpdateDecision(ctx context.Context, requestID uuid.UUID, decision sopdomain.HITLDecision, decidedBy, reason string, version int32) (*sopdomain.HITLRequest, error) {
	row, err := r.queries.UpdateHITLDecision(ctx, db.UpdateHITLDecisionParams{
		RequestID:      requestID,
		Decision:       db.HitlDecision(decision),
		DecidedBy:      pgtype.Text{String: decidedBy, Valid: decidedBy != ""},
		DecisionReason: pgtype.Text{String: reason, Valid: reason != ""},
		Version:        version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.ErrOptimisticLock{ExecutionID: requestID.String()}
		}
		return nil, err
	}
	return mapHITLRequest(row), nil
}

func (r *PostgresHITLRepository) ListByExecution(ctx context.Context, executionID uuid.UUID) ([]sopdomain.HITLRequest, error) {
	rows, err := r.queries.ListHITLRequestsByExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	requests := make([]sopdomain.HITLRequest, len(rows))
	for i, row := range rows {
		requests[i] = *mapHITLRequest(row)
	}
	return requests, nil
}

// mapHITLRequest converts sqlc-generated type to domain type.
func mapHITLRequest(row db.HitlRequest) *sopdomain.HITLRequest {
	req := &sopdomain.HITLRequest{
		RequestID:          row.RequestID,
		SOPExecutionID:     row.SopExecutionID,
		SOPID:              row.SopID,
		TenantID:           row.TenantID,
		StepID:             row.StepID,
		StepName:           row.StepName,
		Decision:           sopdomain.HITLDecision(row.Decision),
		Payload:            row.Payload,
		TemporalWorkflowID: row.TemporalWorkflowID,
		TemporalRunID:      row.TemporalRunID,
		Version:            row.Version,
	}
	if row.DecidedBy.Valid {
		s := row.DecidedBy.String
		req.DecidedBy = &s
	}
	if row.DecisionReason.Valid {
		s := row.DecisionReason.String
		req.DecisionReason = &s
	}
	if row.DecidedAt.Valid {
		t := row.DecidedAt.Time
		req.DecidedAt = &t
	}
	if row.Deadline.Valid {
		req.Deadline = row.Deadline.Time
	}
	if row.CreatedAt.Valid {
		req.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		req.UpdatedAt = row.UpdatedAt.Time
	}
	return req
}

// pgtimeToPointer converts pgtype.Timestamptz to *time.Time.
func pgtimeToPointer(t pgtype.Timestamptz) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}
