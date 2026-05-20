package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/narayana-platform/execution-engine/internal/repository"
	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// AuditService contains the business logic for audit trail queries.
type AuditService struct {
	repo   repository.AuditRepository
	logger zerolog.Logger
}

// NewAuditService creates a new audit service.
func NewAuditService(repo repository.AuditRepository, logger zerolog.Logger) *AuditService {
	return &AuditService{
		repo:   repo,
		logger: logger.With().Str("component", "audit-service").Logger(),
	}
}

// GetAuditTrail returns the full audit trail for an SOP execution.
func (s *AuditService) GetAuditTrail(ctx context.Context, executionID, tenantID uuid.UUID) (*sopdomain.AuditTrailResponse, error) {
	entries, count, err := s.repo.ListByExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}

	responses := make([]sopdomain.AuditEntryResponse, len(entries))
	for i := range entries {
		responses[i] = entries[i].ToResponse()
	}

	return &sopdomain.AuditTrailResponse{
		SOPExecutionID: executionID,
		TenantID:       tenantID,
		Entries:        responses,
		TotalEntries:   int(count),
	}, nil
}
