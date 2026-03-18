package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
	"github.com/narayana-platform/execution-engine/internal/repository/db"
)

// AuditRepository defines the contract for audit trail persistence.
type AuditRepository interface {
	ListByExecution(ctx context.Context, executionID uuid.UUID) ([]sopdomain.AuditEntry, int64, error)
}

// PostgresAuditRepository implements AuditRepository using PostgreSQL via sqlc.
type PostgresAuditRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewPostgresAuditRepository creates a new audit repository.
func NewPostgresAuditRepository(pool *pgxpool.Pool) *PostgresAuditRepository {
	return &PostgresAuditRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *PostgresAuditRepository) ListByExecution(ctx context.Context, executionID uuid.UUID) ([]sopdomain.AuditEntry, int64, error) {
	rows, err := r.queries.ListAuditByExecution(ctx, executionID)
	if err != nil {
		return nil, 0, err
	}

	count, err := r.queries.CountAuditByExecution(ctx, executionID)
	if err != nil {
		return nil, 0, err
	}

	entries := make([]sopdomain.AuditEntry, len(rows))
	for i, row := range rows {
		entries[i] = mapAuditEntry(row)
	}
	return entries, count, nil
}

// mapAuditEntry converts sqlc-generated type to domain type.
func mapAuditEntry(row db.AuditTrail) sopdomain.AuditEntry {
	entry := sopdomain.AuditEntry{
		AuditID:        row.AuditID,
		SOPExecutionID: row.SopExecutionID,
		SOPID:          row.SopID,
		TenantID:       row.TenantID,
		StepID:         row.StepID,
		AgentType:      sopdomain.StepType(row.AgentType),
		Action:         row.Action,
		InputHash:      row.InputHash,
		OutputHash:     row.OutputHash,
		LatencyMs:      row.LatencyMs,
	}
	if row.ModelUsed.Valid {
		s := row.ModelUsed.String
		entry.ModelUsed = &s
	}
	if row.TokensUsed.Valid {
		v := row.TokensUsed.Int32
		entry.TokensUsed = &v
	}
	// Map string compliance tags to ComplianceFramework type
	tags := make([]sopdomain.ComplianceFramework, len(row.ComplianceTags))
	for i, t := range row.ComplianceTags {
		tags[i] = sopdomain.ComplianceFramework(t)
	}
	entry.ComplianceTags = tags

	if row.CreatedAt.Valid {
		entry.CreatedAt = row.CreatedAt.Time
	}
	return entry
}
