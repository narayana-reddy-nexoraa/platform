package workflows

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
	"github.com/narayana-platform/execution-engine/internal/temporal/signals"
)

// SOPWorkflowInput is the input to the generic SOP workflow.
type SOPWorkflowInput struct {
	SOPExecutionID string          `json:"sop_execution_id"`
	SOPID          string          `json:"sop_id"`
	TenantID       string          `json:"tenant_id"`
	Industry       string          `json:"industry"`
	Payload        []byte          `json:"payload"`
	Steps          []StepConfig    `json:"steps"`
}

// StepConfig is a serializable version of AgentStep for Temporal.
type StepConfig struct {
	StepID          string `json:"step_id"`
	StepType        string `json:"step_type"`
	Name            string `json:"name"`
	HITLRequired    bool   `json:"hitl_required"`
	HITLSLASeconds  int    `json:"hitl_sla_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	LLMModel        string `json:"llm_model,omitempty"`
	PromptTemplate  string `json:"prompt_template,omitempty"`
}

// SOPWorkflowOutput is the result of the SOP workflow.
type SOPWorkflowOutput struct {
	Status       string `json:"status"` // "COMPLETED", "REJECTED", "ESCALATED", "FAILED"
	CurrentStep  string `json:"current_step"`
	OutputData   []byte `json:"output_data,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ActivityInput is the common input passed to each activity.
type ActivityInput struct {
	SOPExecutionID string     `json:"sop_execution_id"`
	SOPID          string     `json:"sop_id"`
	TenantID       string     `json:"tenant_id"`
	StepID         string     `json:"step_id"`
	StepType       string     `json:"step_type"`
	Payload        []byte     `json:"payload"`
	StepConfig     StepConfig `json:"step_config"`
}

// DataSources returns data sources from step config, safe for nil.
func (s StepConfig) DataSources() []string {
	return nil // populated from SOP definition at workflow start
}

// TargetSystems returns target systems from step config, safe for nil.
func (s StepConfig) TargetSystems() []string {
	return nil // populated from SOP definition at workflow start
}

// WorkflowInfo holds minimal workflow identity for activities that need it.
type WorkflowInfo struct {
	WorkflowID string
	RunID      string
}

// GetWorkflowInfo extracts workflow identity from activity context.
// Falls back to empty strings if not in a Temporal activity context.
func GetWorkflowInfo(_ context.Context) WorkflowInfo {
	// In real Temporal activity context, use activity.GetInfo(ctx)
	// For now return empty — will be populated via input in production.
	return WorkflowInfo{}
}

// ActivityOutput is the common output from each activity.
type ActivityOutput struct {
	StepID     string `json:"step_id"`
	Success    bool   `json:"success"`
	OutputData []byte `json:"output_data,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SOPWorkflow is the generic Temporal workflow that executes any SOP's 6-agent pipeline.
// It iterates through steps, pauses at HITL gates via Signal, and enforces SLA timers.
func SOPWorkflow(ctx workflow.Context, input SOPWorkflowInput) (*SOPWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("SOP workflow started", "sop_id", input.SOPID, "execution_id", input.SOPExecutionID)

	var lastOutput []byte = input.Payload

	for _, step := range input.Steps {
		logger.Info("Executing step", "step_id", step.StepID, "step_type", step.StepType)

		// Build activity options with per-step timeout and retry
		actOpts := workflow.ActivityOptions{
			StartToCloseTimeout: time.Duration(step.TimeoutSeconds) * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:    time.Second,
				BackoffCoefficient: 2.0,
				MaximumInterval:    30 * time.Second,
				MaximumAttempts:    3,
			},
		}
		actCtx := workflow.WithActivityOptions(ctx, actOpts)

		// Build activity input
		actInput := ActivityInput{
			SOPExecutionID: input.SOPExecutionID,
			SOPID:          input.SOPID,
			TenantID:       input.TenantID,
			StepID:         step.StepID,
			StepType:       step.StepType,
			Payload:        lastOutput,
			StepConfig:     step,
		}

		// Select the correct activity based on step type
		activityName := activityForStepType(step.StepType)

		var actOutput ActivityOutput
		err := workflow.ExecuteActivity(actCtx, activityName, actInput).Get(ctx, &actOutput)
		if err != nil {
			logger.Error("Activity failed", "step_id", step.StepID, "error", err)
			return &SOPWorkflowOutput{
				Status:       "FAILED",
				CurrentStep:  step.StepID,
				ErrorMessage: err.Error(),
			}, nil
		}

		if !actOutput.Success {
			return &SOPWorkflowOutput{
				Status:       "FAILED",
				CurrentStep:  step.StepID,
				ErrorMessage: actOutput.Error,
			}, nil
		}

		// Carry output forward to next step
		if actOutput.OutputData != nil {
			lastOutput = actOutput.OutputData
		}

		// HITL Gate: if this step requires human approval, pause and wait for signal
		if step.HITLRequired {
			logger.Info("HITL gate reached, waiting for approval", "step_id", step.StepID)

			// Create HITL request via activity
			hitlActInput := ActivityInput{
				SOPExecutionID: input.SOPExecutionID,
				SOPID:          input.SOPID,
				TenantID:       input.TenantID,
				StepID:         step.StepID,
				StepType:       string(sopdomain.StepDecisioning),
				Payload:        lastOutput,
				StepConfig:     step,
			}
			var hitlOutput ActivityOutput
			err := workflow.ExecuteActivity(actCtx, "CreateHITLRequest", hitlActInput).Get(ctx, &hitlOutput)
			if err != nil {
				logger.Error("Failed to create HITL request", "error", err)
				return &SOPWorkflowOutput{
					Status:       "FAILED",
					CurrentStep:  step.StepID,
					ErrorMessage: fmt.Sprintf("failed to create HITL request: %v", err),
				}, nil
			}

			// Wait for human signal OR SLA timeout
			slaDuration := time.Duration(step.HITLSLASeconds) * time.Second
			if slaDuration == 0 {
				slaDuration = 2 * time.Hour // default SLA
			}

			var approval signals.HITLApproval
			signalCh := workflow.GetSignalChannel(ctx, signals.HITLSignalName)

			timerCtx, cancelTimer := workflow.WithCancel(ctx)
			timerFuture := workflow.NewTimer(timerCtx, slaDuration)

			selector := workflow.NewSelector(ctx)

			// Branch 1: Human sends approval signal
			selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
				c.Receive(ctx, &approval)
				cancelTimer()
			})

			// Branch 2: SLA timer fires → auto-escalate
			selector.AddFuture(timerFuture, func(f workflow.Future) {
				approval = signals.HITLApproval{
					Decision:  "ESCALATED",
					DecidedBy: "system",
					Reason:    fmt.Sprintf("SLA timeout after %v", slaDuration),
					Timestamp: workflow.Now(ctx),
				}
			})

			selector.Select(ctx)

			logger.Info("HITL decision received", "decision", approval.Decision, "decided_by", approval.DecidedBy)

			switch approval.Decision {
			case "REJECTED":
				return &SOPWorkflowOutput{
					Status:      "REJECTED",
					CurrentStep: step.StepID,
					ErrorMessage: fmt.Sprintf("Rejected by %s: %s", approval.DecidedBy, approval.Reason),
				}, nil
			case "ESCALATED":
				return &SOPWorkflowOutput{
					Status:      "ESCALATED",
					CurrentStep: step.StepID,
					ErrorMessage: approval.Reason,
				}, nil
			}
			// APPROVED → continue to next step
		}
	}

	logger.Info("SOP workflow completed successfully", "sop_id", input.SOPID)

	return &SOPWorkflowOutput{
		Status:     "COMPLETED",
		CurrentStep: "done",
		OutputData: lastOutput,
	}, nil
}

// activityForStepType maps a step type string to the registered activity function name.
func activityForStepType(stepType string) string {
	switch stepType {
	case "INTAKE":
		return "Intake"
	case "DATA_RETRIEVAL":
		return "DataRetrieval"
	case "CLASSIFICATION":
		return "Classification"
	case "DECISIONING":
		return "Decisioning"
	case "EXECUTION":
		return "Execution"
	case "AUDIT":
		return "Audit"
	default:
		return "Intake"
	}
}
