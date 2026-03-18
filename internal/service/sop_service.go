package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"

	"github.com/narayana-platform/execution-engine/internal/domain"
	"github.com/narayana-platform/execution-engine/internal/repository"
	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
	"github.com/narayana-platform/execution-engine/internal/sop/registry"
	"github.com/narayana-platform/execution-engine/internal/temporal/workflows"
)

// SOPService contains the business logic for SOP execution.
type SOPService struct {
	repo           repository.SOPRepository
	registry       *registry.SOPRegistry
	temporalClient client.Client // nil if Temporal is disabled
	logger         zerolog.Logger
}

// NewSOPService creates a new SOP service.
func NewSOPService(
	repo repository.SOPRepository,
	reg *registry.SOPRegistry,
	temporalClient client.Client,
	logger zerolog.Logger,
) *SOPService {
	return &SOPService{
		repo:           repo,
		registry:       reg,
		temporalClient: temporalClient,
		logger:         logger.With().Str("component", "sop-service").Logger(),
	}
}

// StartExecution validates the SOP exists, creates a DB record, and starts a Temporal workflow.
func (s *SOPService) StartExecution(
	ctx context.Context,
	sopID string,
	tenantID uuid.UUID,
	payload json.RawMessage,
) (*sopdomain.SOPExecution, error) {
	// Validate payload
	if !json.Valid(payload) {
		return nil, &domain.ErrValidation{Field: "payload", Message: "must be valid JSON"}
	}

	// Look up SOP definition
	sopDef, err := s.registry.GetByID(sopID)
	if err != nil {
		return nil, &domain.ErrNotFound{Entity: "sop_definition", ID: sopID}
	}

	// Generate execution ID for deterministic Temporal workflow ID
	executionID := uuid.New()
	temporalWorkflowID := sopdomain.TemporalWorkflowID(sopID, executionID)

	var temporalRunID string

	// Start Temporal workflow if client is available
	if s.temporalClient != nil {
		wfInput := buildWorkflowInput(executionID, sopDef, tenantID, payload)

		run, err := s.temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        temporalWorkflowID,
			TaskQueue: sopDef.TaskQueue(),
		}, workflows.SOPWorkflow, wfInput)
		if err != nil {
			return nil, fmt.Errorf("start temporal workflow: %w", err)
		}
		temporalRunID = run.GetRunID()

		s.logger.Info().
			Str("sop_id", sopID).
			Str("workflow_id", temporalWorkflowID).
			Str("run_id", temporalRunID).
			Str("task_queue", sopDef.TaskQueue()).
			Msg("temporal workflow started")
	}

	// Create DB record
	exec, err := s.repo.Create(ctx, sopID, tenantID, string(sopDef.Industry), payload, temporalWorkflowID, temporalRunID)
	if err != nil {
		return nil, fmt.Errorf("create sop execution: %w", err)
	}

	return exec, nil
}

// GetExecution retrieves an SOP execution by ID, scoped to a tenant.
func (s *SOPService) GetExecution(ctx context.Context, executionID, tenantID uuid.UUID) (*sopdomain.SOPExecution, error) {
	return s.repo.GetByID(ctx, executionID, tenantID)
}

// ListExecutions returns a paginated list of SOP executions.
func (s *SOPService) ListExecutions(
	ctx context.Context,
	tenantID uuid.UUID,
	sopID, status, industry *string,
	limit, offset int32,
) (*sopdomain.SOPPaginatedResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	executions, totalCount, err := s.repo.List(ctx, tenantID, sopID, status, industry, limit, offset)
	if err != nil {
		return nil, err
	}

	data := make([]sopdomain.SOPExecutionResponse, len(executions))
	for i := range executions {
		data[i] = executions[i].ToResponse()
	}

	return &sopdomain.SOPPaginatedResponse{
		Data:       data,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

// buildWorkflowInput converts a SOP definition + payload into a Temporal workflow input.
func buildWorkflowInput(executionID uuid.UUID, def *sopdomain.SOPDefinition, tenantID uuid.UUID, payload json.RawMessage) workflows.SOPWorkflowInput {
	steps := make([]workflows.StepConfig, len(def.Steps))
	for i, s := range def.Steps {
		steps[i] = workflows.StepConfig{
			StepID:         s.StepID,
			StepType:       string(s.StepType),
			Name:           s.Name,
			HITLRequired:   s.HITLRequired,
			HITLSLASeconds: int(s.HITLSLADuration.Seconds()),
			TimeoutSeconds: int(s.Timeout.Seconds()),
			LLMModel:       s.Config.LLMModel,
			PromptTemplate: s.Config.PromptTemplate,
		}
	}

	return workflows.SOPWorkflowInput{
		SOPExecutionID: executionID.String(),
		SOPID:          def.SOPID,
		TenantID:       tenantID.String(),
		Industry:       string(def.Industry),
		Payload:        payload,
		Steps:          steps,
	}
}
