package registry

import (
	"time"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// StepOverrides allows per-SOP customization of the standard 6-step pattern.
type StepOverrides struct {
	// IntakeName overrides the intake step name (default: "Intake and Validation").
	IntakeName string
	IntakeDesc string

	// DataRetrievalName overrides the data retrieval step name.
	DataRetrievalName string
	DataRetrievalDesc string
	DataSources       []string

	// ClassificationName overrides the classification step name.
	ClassificationName string
	ClassificationDesc string
	LLMModel           string
	PromptTemplate     string

	// ClassificationFewShots provides domain-specific examples for the classification LLM.
	// Pattern: AgentGoal.example_conversation_history from Temporal reinsurance case study.
	ClassificationFewShots []sopdomain.FewShotExample

	// DecisioningName overrides the decisioning step name.
	DecisioningName string
	DecisioningDesc string
	// HITLAfterDecisioning controls whether a HITL gate follows the decisioning step.
	HITLAfterDecisioning bool
	HITLSLADuration      time.Duration

	// DecisioningFewShots provides domain-specific examples for the decisioning LLM.
	DecisioningFewShots []sopdomain.FewShotExample

	// ExecutionName overrides the execution step name.
	ExecutionName string
	ExecutionDesc string
	TargetSystems []string

	// AuditName overrides the audit step name.
	AuditName string
	AuditDesc string

	// MaxContextTokens limits LLM context per step (0 = unlimited).
	MaxContextTokens int
}

// BuildStandardSteps creates the 6-step agent pattern with per-SOP overrides.
func BuildStandardSteps(o StepOverrides) []sopdomain.AgentStep {
	defaultModel := "gpt-4o-mini"
	if o.LLMModel != "" {
		defaultModel = o.LLMModel
	}

	defaultHITLSLA := 2 * time.Hour
	if o.HITLSLADuration > 0 {
		defaultHITLSLA = o.HITLSLADuration
	}

	return []sopdomain.AgentStep{
		{
			StepID:      "intake",
			StepType:    sopdomain.StepIntake,
			Name:        withDefault(o.IntakeName, "Intake and Validation"),
			Description: withDefault(o.IntakeDesc, "Receive, parse, and validate incoming data"),
			Timeout:     30 * time.Second,
			RetryPolicy: sopdomain.DefaultRetryPolicy(),
			Config:      sopdomain.StepConfig{},
		},
		{
			StepID:      "data_retrieval",
			StepType:    sopdomain.StepDataRetrieval,
			Name:        withDefault(o.DataRetrievalName, "Data Retrieval"),
			Description: withDefault(o.DataRetrievalDesc, "Fetch data from external systems and APIs"),
			Timeout:     60 * time.Second,
			RetryPolicy: sopdomain.DefaultRetryPolicy(),
			Config: sopdomain.StepConfig{
				DataSources: o.DataSources,
			},
		},
		{
			StepID:      "classification",
			StepType:    sopdomain.StepClassification,
			Name:        withDefault(o.ClassificationName, "Classification and Triage"),
			Description: withDefault(o.ClassificationDesc, "Categorize, score risk, and prioritize"),
			Timeout:     45 * time.Second,
			RetryPolicy: sopdomain.DefaultRetryPolicy(),
			Config: sopdomain.StepConfig{
				LLMModel:         defaultModel,
				PromptTemplate:   o.PromptTemplate,
				FewShotExamples:  o.ClassificationFewShots,
				MaxContextTokens: o.MaxContextTokens,
			},
		},
		{
			StepID:          "decisioning",
			StepType:        sopdomain.StepDecisioning,
			Name:            withDefault(o.DecisioningName, "Decisioning"),
			Description:     withDefault(o.DecisioningDesc, "Apply business rules and recommend action"),
			HITLRequired:    o.HITLAfterDecisioning,
			HITLSLADuration: defaultHITLSLA,
			Timeout:         45 * time.Second,
			RetryPolicy:     sopdomain.DefaultRetryPolicy(),
			Config: sopdomain.StepConfig{
				LLMModel:         defaultModel,
				PromptTemplate:   o.PromptTemplate,
				FewShotExamples:  o.DecisioningFewShots,
				MaxContextTokens: o.MaxContextTokens,
			},
		},
		{
			StepID:      "execution",
			StepType:    sopdomain.StepExecution,
			Name:        withDefault(o.ExecutionName, "Execution"),
			Description: withDefault(o.ExecutionDesc, "Execute the decided action in target systems"),
			Timeout:     60 * time.Second,
			RetryPolicy: sopdomain.DefaultRetryPolicy(),
			Config: sopdomain.StepConfig{
				TargetSystems: o.TargetSystems,
			},
		},
		{
			StepID:      "audit",
			StepType:    sopdomain.StepAudit,
			Name:        withDefault(o.AuditName, "Audit and Evidence"),
			Description: withDefault(o.AuditDesc, "Log all actions immutably with compliance evidence"),
			Timeout:     15 * time.Second,
			RetryPolicy: sopdomain.DefaultRetryPolicy(),
			Config:      sopdomain.StepConfig{},
		},
	}
}

func withDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
