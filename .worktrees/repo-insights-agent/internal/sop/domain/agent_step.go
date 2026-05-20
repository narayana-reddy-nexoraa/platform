package domain

import "time"

// StepType identifies the role of an agent in the SOP workflow.
type StepType string

const (
	StepIntake          StepType = "INTAKE"
	StepDataRetrieval   StepType = "DATA_RETRIEVAL"
	StepClassification  StepType = "CLASSIFICATION"
	StepDecisioning     StepType = "DECISIONING"
	StepExecution       StepType = "EXECUTION"
	StepAudit           StepType = "AUDIT"
)

// AgentStep defines a single step in an SOP workflow.
type AgentStep struct {
	// StepID uniquely identifies this step within the SOP (e.g., "intake", "triage").
	StepID string `json:"step_id"`

	// StepType is the agent role for this step.
	StepType StepType `json:"step_type"`

	// Name is a human-readable label (e.g., "Policy Verification").
	Name string `json:"name"`

	// Description explains what this step does.
	Description string `json:"description"`

	// HITLRequired indicates a human-in-the-loop gate after this step.
	HITLRequired bool `json:"hitl_required"`

	// HITLSLADuration is the max time to wait for human approval before auto-escalation.
	// Only relevant when HITLRequired is true.
	HITLSLADuration time.Duration `json:"hitl_sla_duration,omitempty"`

	// Timeout is the max execution time for this step's activity.
	Timeout time.Duration `json:"timeout"`

	// RetryPolicy controls retry behavior on transient failures.
	RetryPolicy StepRetryPolicy `json:"retry_policy"`

	// Config holds step-specific configuration (LLM model, prompt template, API endpoint, etc.).
	Config StepConfig `json:"config"`
}

// StepRetryPolicy controls how a failed activity is retried.
type StepRetryPolicy struct {
	MaxAttempts       int32         `json:"max_attempts"`
	InitialInterval   time.Duration `json:"initial_interval"`
	MaxInterval       time.Duration `json:"max_interval"`
	BackoffCoefficient float64      `json:"backoff_coefficient"`
}

// DefaultRetryPolicy returns a sensible default retry policy.
func DefaultRetryPolicy() StepRetryPolicy {
	return StepRetryPolicy{
		MaxAttempts:        3,
		InitialInterval:    time.Second,
		MaxInterval:        30 * time.Second,
		BackoffCoefficient: 2.0,
	}
}

// StepConfig holds agent-specific configuration for a workflow step.
// Inspired by the AgentGoal pattern from the Temporal reinsurance case study
// (Sophia Barnes, Stanford, Jan 2026) — each step is a focused "agent goal"
// with scoped context, few-shot examples, and simple tool arguments.
type StepConfig struct {
	// LLMModel is the model ID for classification/decisioning steps (e.g., "gpt-4o-mini", "claude-sonnet-4-6").
	LLMModel string `json:"llm_model,omitempty"`

	// PromptTemplate is the prompt template key from the prompt registry.
	PromptTemplate string `json:"prompt_template,omitempty"`

	// FewShotExamples provides domain-specific example conversations that improve
	// LLM accuracy for this step. Each example is an input/output pair the LLM
	// can reference when making decisions.
	// Pattern: AgentGoal.example_conversation_history from Temporal reinsurance case study.
	FewShotExamples []FewShotExample `json:"few_shot_examples,omitempty"`

	// DataSources lists external system references for data retrieval steps.
	DataSources []string `json:"data_sources,omitempty"`

	// ValidationSchema is a JSON Schema reference for input/output validation.
	ValidationSchema string `json:"validation_schema,omitempty"`

	// TargetSystems lists systems to write to for execution steps.
	TargetSystems []string `json:"target_systems,omitempty"`

	// MaxContextTokens limits the context window passed to the LLM for this step.
	// Prevents token bloat when dealing with large inter-step data.
	// Pattern: context isolation from Temporal reinsurance case study.
	MaxContextTokens int `json:"max_context_tokens,omitempty"`
}

// FewShotExample is an input/output pair used to prime the LLM for domain-specific tasks.
// Pattern: AgentGoal.example_conversation_history from Temporal reinsurance case study.
type FewShotExample struct {
	// Input is the example user/system input.
	Input string `json:"input"`

	// Output is the expected agent response/action for this input.
	Output string `json:"output"`

	// Explanation optionally describes why this output is correct (for auditability).
	Explanation string `json:"explanation,omitempty"`
}
