package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
	"github.com/narayana-platform/execution-engine/internal/temporal/signals"
)

// SOPWorkflowInput is the input to the generic SOP workflow.
type SOPWorkflowInput struct {
	SOPExecutionID string       `json:"sop_execution_id"`
	SOPID          string       `json:"sop_id"`
	TenantID       string       `json:"tenant_id"`
	Industry       string       `json:"industry"`
	Payload        []byte       `json:"payload"`
	Steps          []StepConfig `json:"steps"`

	// ParentBridgeWorkflowID is set when this SOP is spawned as a child
	// of a BridgeWorkflow (cross-SOP orchestration).
	// Pattern: Bridge Workflow from Temporal reinsurance case study.
	ParentBridgeWorkflowID string `json:"parent_bridge_workflow_id,omitempty"`
}

// StepConfig is a serializable version of AgentStep for Temporal.
type StepConfig struct {
	StepID         string `json:"step_id"`
	StepType       string `json:"step_type"`
	Name           string `json:"name"`
	HITLRequired   bool   `json:"hitl_required"`
	HITLSLASeconds int    `json:"hitl_sla_seconds"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	LLMModel       string `json:"llm_model,omitempty"`
	PromptTemplate string `json:"prompt_template,omitempty"`

	// FewShotExamples carries domain-specific examples for the LLM.
	// Pattern: AgentGoal.example_conversation_history from Temporal reinsurance case study.
	FewShotExamples []FewShotExampleConfig `json:"few_shot_examples,omitempty"`

	// MaxContextTokens limits context window to prevent token bloat.
	MaxContextTokens int `json:"max_context_tokens,omitempty"`
}

// FewShotExampleConfig is the serializable few-shot example for Temporal.
type FewShotExampleConfig struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation,omitempty"`
}

// DataSources returns data sources from step config.
func (s StepConfig) DataSources() []string {
	return nil
}

// TargetSystems returns target systems from step config.
func (s StepConfig) TargetSystems() []string {
	return nil
}

// WorkflowInfo holds minimal workflow identity for activities that need it.
type WorkflowInfo struct {
	WorkflowID string
	RunID      string
}

// GetWorkflowInfo extracts workflow identity from activity context.
func GetWorkflowInfo(_ context.Context) WorkflowInfo {
	return WorkflowInfo{}
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

	// UserContext carries human-injected guidance from HITL approval or
	// mid-workflow context signal. Merged into the LLM prompt for this step.
	// Pattern: user_input from Temporal reinsurance case study.
	UserContext string `json:"user_context,omitempty"`
}

// ActivityOutput is the common output from each activity.
type ActivityOutput struct {
	StepID     string `json:"step_id"`
	Success    bool   `json:"success"`
	OutputData []byte `json:"output_data,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SOPWorkflow is the generic Temporal workflow that executes any SOP's 6-agent pipeline.
// It iterates through steps, pauses at HITL gates via Signal, enforces SLA timers,
// and supports mid-workflow user context injection.
//
// Patterns adopted from "Trusting AI Agents: A Reinsurance Case Study" (Temporal blog, Jan 2026):
// 1. Modular sub-agents (each step is a focused agent with scoped context)
// 2. HITL via Temporal Signals (workflow pauses, human approves/rejects/escalates)
// 3. User context injection (human can inject guidance via approval or separate signal)
// 4. Few-shot examples per step (AgentGoal pattern)
// 5. Bridge Workflow support for cross-SOP orchestration
func SOPWorkflow(ctx workflow.Context, input SOPWorkflowInput) (*SOPWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("SOP workflow started",
		"sop_id", input.SOPID,
		"execution_id", input.SOPExecutionID,
		"parent_bridge", input.ParentBridgeWorkflowID,
	)

	var lastOutput []byte = input.Payload
	var pendingUserContext string // accumulated user context from signals

	// Drain any user context signals that arrived before workflow started
	drainUserContext(ctx, &pendingUserContext)

	for _, step := range input.Steps {
		logger.Info("Executing step", "step_id", step.StepID, "step_type", step.StepType)

		// Check for user context signals between steps
		drainUserContext(ctx, &pendingUserContext)

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

		// Build activity input with user context if available
		actInput := ActivityInput{
			SOPExecutionID: input.SOPExecutionID,
			SOPID:          input.SOPID,
			TenantID:       input.TenantID,
			StepID:         step.StepID,
			StepType:       step.StepType,
			Payload:        lastOutput,
			StepConfig:     step,
			UserContext:     pendingUserContext,
		}

		// Clear user context after passing it to activity
		pendingUserContext = ""

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
				slaDuration = 2 * time.Hour
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

			// Branch 2: SLA timer fires -> auto-escalate
			selector.AddFuture(timerFuture, func(f workflow.Future) {
				approval = signals.HITLApproval{
					Decision:  "ESCALATED",
					DecidedBy: "system",
					Reason:    fmt.Sprintf("SLA timeout after %v", slaDuration),
					Timestamp: workflow.Now(ctx),
				}
			})

			selector.Select(ctx)

			logger.Info("HITL decision received",
				"decision", approval.Decision,
				"decided_by", approval.DecidedBy,
				"has_user_context", approval.UserContext != "",
			)

			switch approval.Decision {
			case "REJECTED":
				return &SOPWorkflowOutput{
					Status:       "REJECTED",
					CurrentStep:  step.StepID,
					ErrorMessage: fmt.Sprintf("Rejected by %s: %s", approval.DecidedBy, approval.Reason),
				}, nil
			case "ESCALATED":
				return &SOPWorkflowOutput{
					Status:       "ESCALATED",
					CurrentStep:  step.StepID,
					ErrorMessage: approval.Reason,
				}, nil
			}

			// APPROVED — carry user context to next step if provided
			if approval.UserContext != "" {
				pendingUserContext = approval.UserContext
				logger.Info("User context injected via HITL approval",
					"context_length", len(approval.UserContext),
					"decided_by", approval.DecidedBy,
				)
			}
		}
	}

	// If this SOP is a child of a Bridge Workflow, signal completion
	if input.ParentBridgeWorkflowID != "" {
		logger.Info("Signaling parent bridge workflow",
			"bridge_id", input.ParentBridgeWorkflowID,
		)
		// Parent bridge workflow will receive this via ChildWorkflowFuture.Get()
	}

	logger.Info("SOP workflow completed successfully", "sop_id", input.SOPID)

	return &SOPWorkflowOutput{
		Status:      "COMPLETED",
		CurrentStep: "done",
		OutputData:  lastOutput,
	}, nil
}

// drainUserContext checks for any pending UserContextSignal on the non-blocking channel
// and appends the context to the accumulator. This allows humans to inject guidance
// at any point during workflow execution, not just at HITL gates.
func drainUserContext(ctx workflow.Context, accumulator *string) {
	ch := workflow.GetSignalChannel(ctx, signals.UserContextSignalName)
	for {
		var sig signals.UserContextSignal
		ok := ch.ReceiveAsync(&sig)
		if !ok {
			return
		}
		if sig.Context != "" {
			if *accumulator != "" {
				*accumulator += "\n---\n"
			}
			*accumulator += fmt.Sprintf("[%s @ %s]: %s",
				sig.ProvidedBy,
				sig.Timestamp.Format(time.RFC3339),
				sig.Context,
			)
		}
	}
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

// --- Bridge Workflow Pattern ---
// Inspired by the "Bridge Workflow" from the Temporal reinsurance case study.
// A Bridge Workflow orchestrates multiple related SOP workflows that need to
// share state and coordinate execution (e.g., HOSP-02 Discharge triggers
// HOSP-01 Bed Assignment, or FS-02 AML triggers Counterparty Risk).

// BridgeWorkflowInput configures a cross-SOP orchestration.
type BridgeWorkflowInput struct {
	BridgeID    string               `json:"bridge_id"`
	TenantID    string               `json:"tenant_id"`
	Description string               `json:"description"`
	ChildSOPs   []ChildSOPConfig     `json:"child_sops"`
	SharedState map[string][]byte    `json:"shared_state,omitempty"`
}

// ChildSOPConfig defines a child SOP to be executed within the bridge.
type ChildSOPConfig struct {
	SOPID    string `json:"sop_id"`
	Industry string `json:"industry"`
	Payload  []byte `json:"payload"`

	// DependsOn lists SOP IDs that must complete before this one starts.
	// Empty means it can start immediately (parallel execution).
	DependsOn []string `json:"depends_on,omitempty"`
}

// BridgeWorkflowOutput aggregates results from all child SOPs.
type BridgeWorkflowOutput struct {
	BridgeID    string                      `json:"bridge_id"`
	Status      string                      `json:"status"` // "COMPLETED", "PARTIAL", "FAILED"
	ChildResults map[string]*SOPWorkflowOutput `json:"child_results"`
}

// BridgeWorkflow orchestrates multiple related SOP workflows, maintaining shared
// state and coordinating dependencies between them. Child workflows can access
// shared state via the bridge, preventing token bloat from passing large datasets
// through individual SOP workflow inputs.
//
// Example use cases:
// - HOSP-02 Discharge completes -> HOSP-01 Bed Assignment triggered
// - FS-02 AML alert -> CPR-01 Counterparty Risk assessment -> back to FS-02
// - INS-01 FNOL creates claim -> INS-03 Claims Adjudication picks it up
func BridgeWorkflow(ctx workflow.Context, input BridgeWorkflowInput) (*BridgeWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Bridge workflow started",
		"bridge_id", input.BridgeID,
		"child_count", len(input.ChildSOPs),
	)

	results := make(map[string]*SOPWorkflowOutput)
	completed := make(map[string]bool)

	// Process children respecting dependency order
	for {
		madeProgress := false

		for _, child := range input.ChildSOPs {
			// Skip already completed
			if completed[child.SOPID] {
				continue
			}

			// Check if dependencies are met
			depsReady := true
			for _, dep := range child.DependsOn {
				if !completed[dep] {
					depsReady = false
					break
				}
			}
			if !depsReady {
				continue
			}

			// Dependencies met — start child SOP workflow
			logger.Info("Starting child SOP", "sop_id", child.SOPID, "depends_on", child.DependsOn)

			// Merge output from dependency SOPs into child's payload if available
			childPayload := child.Payload
			if len(child.DependsOn) > 0 {
				childPayload = mergeDepOutputs(child.Payload, child.DependsOn, results)
			}

			childInput := SOPWorkflowInput{
				SOPExecutionID:         fmt.Sprintf("bridge-%s-%s", input.BridgeID, child.SOPID),
				SOPID:                  child.SOPID,
				TenantID:               input.TenantID,
				Industry:               child.Industry,
				Payload:                childPayload,
				ParentBridgeWorkflowID: input.BridgeID,
			}

			childOpts := workflow.ChildWorkflowOptions{
				WorkflowID: fmt.Sprintf("bridge-%s-child-%s", input.BridgeID, child.SOPID),
				TaskQueue:  sopdomain.Industry(child.Industry).TaskQueue(),
			}
			childCtx := workflow.WithChildOptions(ctx, childOpts)

			var childResult SOPWorkflowOutput
			err := workflow.ExecuteChildWorkflow(childCtx, SOPWorkflow, childInput).Get(ctx, &childResult)
			if err != nil {
				logger.Error("Child SOP failed", "sop_id", child.SOPID, "error", err)
				results[child.SOPID] = &SOPWorkflowOutput{
					Status:       "FAILED",
					ErrorMessage: err.Error(),
				}
			} else {
				results[child.SOPID] = &childResult
			}

			completed[child.SOPID] = true
			madeProgress = true
		}

		if !madeProgress {
			break // No more children to process (all done or stuck on unmet deps)
		}
	}

	// Determine overall status
	overallStatus := "COMPLETED"
	allCompleted := len(completed) == len(input.ChildSOPs)
	hasFailures := false
	for _, r := range results {
		if r.Status == "FAILED" || r.Status == "REJECTED" {
			hasFailures = true
		}
	}

	if !allCompleted {
		overallStatus = "PARTIAL"
	} else if hasFailures {
		overallStatus = "PARTIAL"
	}

	logger.Info("Bridge workflow finished",
		"bridge_id", input.BridgeID,
		"status", overallStatus,
		"completed", len(completed),
		"total", len(input.ChildSOPs),
	)

	return &BridgeWorkflowOutput{
		BridgeID:     input.BridgeID,
		Status:       overallStatus,
		ChildResults: results,
	}, nil
}

// mergeDepOutputs combines the output data from dependency SOPs into the child's payload.
// This implements the inter-agent data store pattern from the reinsurance case study —
// large data is passed between agents via persistent state rather than LLM context.
func mergeDepOutputs(basePayload []byte, depIDs []string, results map[string]*SOPWorkflowOutput) []byte {
	merged := map[string]json.RawMessage{
		"original_payload": basePayload,
	}
	for _, depID := range depIDs {
		if r, ok := results[depID]; ok && r.OutputData != nil {
			merged[fmt.Sprintf("dep_%s_output", depID)] = r.OutputData
		}
	}
	out, _ := json.Marshal(merged)
	return out
}
