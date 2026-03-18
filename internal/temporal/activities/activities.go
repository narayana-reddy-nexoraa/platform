package activities

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/activity"

	"github.com/narayana-platform/execution-engine/internal/repository/db"
	"github.com/narayana-platform/execution-engine/internal/temporal/workflows"
)

// Activities holds shared dependencies for all Temporal activities.
type Activities struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  zerolog.Logger
}

// NewActivities creates a new Activities instance with database access.
func NewActivities(pool *pgxpool.Pool, logger zerolog.Logger) *Activities {
	return &Activities{
		pool:    pool,
		queries: db.New(pool),
		logger:  logger.With().Str("component", "activities").Logger(),
	}
}

// Intake parses and validates the incoming data for an SOP execution step.
func (a *Activities) Intake(ctx context.Context, input workflows.ActivityInput) (*workflows.ActivityOutput, error) {
	a.logger.Info().
		Str("sop_id", input.SOPID).
		Str("step_id", input.StepID).
		Str("execution_id", input.SOPExecutionID).
		Msg("intake activity started")

	start := time.Now()

	// Validate that payload is valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(input.Payload, &parsed); err != nil {
		return &workflows.ActivityOutput{
			StepID:  input.StepID,
			Success: false,
			Error:   fmt.Sprintf("invalid JSON payload: %v", err),
		}, nil
	}

	a.logAudit(ctx, input, start)

	return &workflows.ActivityOutput{
		StepID:     input.StepID,
		Success:    true,
		OutputData: input.Payload,
	}, nil
}

// DataRetrieval fetches data from external systems and APIs.
func (a *Activities) DataRetrieval(ctx context.Context, input workflows.ActivityInput) (*workflows.ActivityOutput, error) {
	a.logger.Info().
		Str("sop_id", input.SOPID).
		Str("step_id", input.StepID).
		Str("execution_id", input.SOPExecutionID).
		Msg("data retrieval activity started")

	start := time.Now()

	// TODO: Implement actual data retrieval from configured data sources.
	enriched := map[string]interface{}{
		"original_payload": json.RawMessage(input.Payload),
		"retrieval_metadata": map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"status":    "simulated",
		},
	}
	output, _ := json.Marshal(enriched)

	a.logAudit(ctx, input, start)

	return &workflows.ActivityOutput{
		StepID:     input.StepID,
		Success:    true,
		OutputData: output,
	}, nil
}

// Classification categorizes, scores risk, and prioritizes the data.
func (a *Activities) Classification(ctx context.Context, input workflows.ActivityInput) (*workflows.ActivityOutput, error) {
	a.logger.Info().
		Str("sop_id", input.SOPID).
		Str("step_id", input.StepID).
		Str("execution_id", input.SOPExecutionID).
		Msg("classification activity started")

	start := time.Now()

	// TODO: Implement actual LLM-based classification.
	result := map[string]interface{}{
		"input_data": json.RawMessage(input.Payload),
		"classification": map[string]interface{}{
			"risk_level": "MEDIUM",
			"confidence": 0.85,
			"category":   "standard",
			"model_used": input.StepConfig.LLMModel,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	}
	output, _ := json.Marshal(result)

	a.logAudit(ctx, input, start)

	return &workflows.ActivityOutput{
		StepID:     input.StepID,
		Success:    true,
		OutputData: output,
	}, nil
}

// Decisioning applies business rules and recommends an action.
func (a *Activities) Decisioning(ctx context.Context, input workflows.ActivityInput) (*workflows.ActivityOutput, error) {
	a.logger.Info().
		Str("sop_id", input.SOPID).
		Str("step_id", input.StepID).
		Str("execution_id", input.SOPExecutionID).
		Msg("decisioning activity started")

	start := time.Now()

	// TODO: Implement actual LLM-based decisioning with business rules.
	decision := map[string]interface{}{
		"input_data": json.RawMessage(input.Payload),
		"decision": map[string]interface{}{
			"action":     "proceed",
			"confidence": 0.90,
			"rationale":  "All criteria met per SOP guidelines",
			"model_used": input.StepConfig.LLMModel,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		},
	}
	output, _ := json.Marshal(decision)

	a.logAudit(ctx, input, start)

	return &workflows.ActivityOutput{
		StepID:     input.StepID,
		Success:    true,
		OutputData: output,
	}, nil
}

// CreateHITLRequest creates a human-in-the-loop approval request in the database.
func (a *Activities) CreateHITLRequest(ctx context.Context, input workflows.ActivityInput) (*workflows.ActivityOutput, error) {
	a.logger.Info().
		Str("sop_id", input.SOPID).
		Str("step_id", input.StepID).
		Str("execution_id", input.SOPExecutionID).
		Msg("creating HITL request")

	execID, err := uuid.Parse(input.SOPExecutionID)
	if err != nil {
		return nil, fmt.Errorf("invalid execution ID: %w", err)
	}

	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Calculate deadline from SLA
	slaDuration := time.Duration(input.StepConfig.HITLSLASeconds) * time.Second
	if slaDuration == 0 {
		slaDuration = 2 * time.Hour
	}
	deadline := time.Now().Add(slaDuration)

	// Get workflow identity from Temporal activity context
	info := activity.GetInfo(ctx)

	_, err = a.queries.CreateHITLRequest(ctx, db.CreateHITLRequestParams{
		SopExecutionID: execID,
		SopID:          input.SOPID,
		TenantID:       tenantID,
		StepID:         input.StepID,
		StepName:       input.StepConfig.Name,
		Deadline:       pgtype.Timestamptz{Time: deadline, Valid: true},
		Payload:        input.Payload,
		TemporalWorkflowID: info.WorkflowExecution.ID,
		TemporalRunID:      info.WorkflowExecution.RunID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create HITL request: %w", err)
	}

	return &workflows.ActivityOutput{
		StepID:  input.StepID,
		Success: true,
	}, nil
}

// Execution performs the decided action in target systems.
func (a *Activities) Execution(ctx context.Context, input workflows.ActivityInput) (*workflows.ActivityOutput, error) {
	a.logger.Info().
		Str("sop_id", input.SOPID).
		Str("step_id", input.StepID).
		Str("execution_id", input.SOPExecutionID).
		Msg("execution activity started")

	start := time.Now()

	// TODO: Implement actual system writes (CRM, EHR, Claims System, etc.).
	result := map[string]interface{}{
		"input_data": json.RawMessage(input.Payload),
		"execution_result": map[string]interface{}{
			"status":    "executed",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	}
	output, _ := json.Marshal(result)

	a.logAudit(ctx, input, start)

	return &workflows.ActivityOutput{
		StepID:     input.StepID,
		Success:    true,
		OutputData: output,
	}, nil
}

// Audit writes an immutable audit trail entry with compliance evidence.
func (a *Activities) Audit(ctx context.Context, input workflows.ActivityInput) (*workflows.ActivityOutput, error) {
	a.logger.Info().
		Str("sop_id", input.SOPID).
		Str("step_id", input.StepID).
		Str("execution_id", input.SOPExecutionID).
		Msg("audit activity — final step")

	start := time.Now()
	a.logAudit(ctx, input, start)

	return &workflows.ActivityOutput{
		StepID:  input.StepID,
		Success: true,
	}, nil
}

// logAudit inserts an audit trail entry into the database.
func (a *Activities) logAudit(ctx context.Context, input workflows.ActivityInput, start time.Time) {
	execID, err := uuid.Parse(input.SOPExecutionID)
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to parse execution ID for audit")
		return
	}

	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		a.logger.Error().Err(err).Msg("failed to parse tenant ID for audit")
		return
	}

	inputHash := fmt.Sprintf("%x", sha256.Sum256(input.Payload))
	outputHash := inputHash
	latency := time.Since(start).Milliseconds()

	_, err = a.queries.InsertAuditEntry(ctx, db.InsertAuditEntryParams{
		SopExecutionID: execID,
		SopID:          input.SOPID,
		TenantID:       tenantID,
		StepID:         input.StepID,
		AgentType:      db.AgentStepType(input.StepType),
		Action:         fmt.Sprintf("%s.execute", input.StepID),
		InputHash:      inputHash,
		OutputHash:     outputHash,
		ModelUsed:      pgtype.Text{String: input.StepConfig.LLMModel, Valid: input.StepConfig.LLMModel != ""},
		LatencyMs:      latency,
		TokensUsed:     pgtype.Int4{Valid: false},
		ComplianceTags: []string{},
	})
	if err != nil {
		a.logger.Error().Err(err).Str("step_id", input.StepID).Msg("failed to insert audit entry")
	}
}
