package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
	"github.com/narayana-platform/execution-engine/internal/temporal/signals"
)

// HITLService contains the business logic for human-in-the-loop approvals.
type HITLService struct {
	repo           repository.HITLRepository
	temporalClient client.Client
	logger         zerolog.Logger
}

// NewHITLService creates a new HITL service.
func NewHITLService(
	repo repository.HITLRepository,
	temporalClient client.Client,
	logger zerolog.Logger,
) *HITLService {
	return &HITLService{
		repo:           repo,
		temporalClient: temporalClient,
		logger:         logger.With().Str("component", "hitl-service").Logger(),
	}
}

// GetRequest retrieves an HITL request by ID, scoped to a tenant.
func (s *HITLService) GetRequest(ctx context.Context, requestID, tenantID uuid.UUID) (*sopdomain.HITLRequest, error) {
	return s.repo.GetByID(ctx, requestID, tenantID)
}

// ListPending returns a paginated list of pending HITL requests for a tenant.
func (s *HITLService) ListPending(
	ctx context.Context,
	tenantID uuid.UUID,
	limit, offset int32,
) ([]sopdomain.HITLRequest, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.repo.ListPending(ctx, tenantID, limit, offset)
}

// Decide updates the HITL decision in the DB and signals the paused Temporal workflow.
func (s *HITLService) Decide(
	ctx context.Context,
	requestID, tenantID uuid.UUID,
	req sopdomain.HITLDecideRequest,
) (*sopdomain.HITLRequest, error) {
	// Validate decision
	switch req.Decision {
	case sopdomain.HITLApproved, sopdomain.HITLRejected, sopdomain.HITLEscalated:
		// valid
	default:
		return nil, &domain.ErrValidation{Field: "decision", Message: "must be APPROVED, REJECTED, or ESCALATED"}
	}

	// Fetch current request to get version and workflow info
	current, err := s.repo.GetByID(ctx, requestID, tenantID)
	if err != nil {
		return nil, err
	}

	if current.Decision != sopdomain.HITLPending {
		return nil, &domain.ErrValidation{
			Field:   "decision",
			Message: fmt.Sprintf("request already decided: %s", current.Decision),
		}
	}

	// Update decision in DB
	updated, err := s.repo.UpdateDecision(ctx, requestID, req.Decision, req.DecidedBy, req.Reason, current.Version)
	if err != nil {
		return nil, err
	}

	// Signal the paused Temporal workflow
	if s.temporalClient != nil && current.TemporalWorkflowID != "" {
		signal := signals.HITLApproval{
			Decision:  string(req.Decision),
			DecidedBy: req.DecidedBy,
			Reason:    req.Reason,
			Timestamp: time.Now(),
		}

		err := s.temporalClient.SignalWorkflow(ctx, current.TemporalWorkflowID, current.TemporalRunID, signals.HITLSignalName, signal)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("workflow_id", current.TemporalWorkflowID).
				Str("request_id", requestID.String()).
				Msg("failed to signal temporal workflow — decision saved but workflow not resumed")
			// Don't fail the request — the decision is saved. The workflow will
			// eventually timeout and auto-escalate if the signal is lost.
		} else {
			s.logger.Info().
				Str("request_id", requestID.String()).
				Str("decision", string(req.Decision)).
				Str("workflow_id", current.TemporalWorkflowID).
				Msg("temporal workflow signaled with HITL decision")
		}
	}

	return updated, nil
}
