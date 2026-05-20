package signals

import "time"

// HITLSignalName is the Temporal signal channel name for HITL approval.
const HITLSignalName = "hitl-approval"

// UserContextSignalName is the signal channel for injecting human context mid-workflow.
// Pattern: user_input parameter from Temporal reinsurance case study —
// allows humans to inject guidance for edge cases the LLM can't handle alone.
const UserContextSignalName = "user-context"

// HITLApproval is the signal payload sent by the HITL UI when a human makes a decision.
type HITLApproval struct {
	Decision  string    `json:"decision"`  // "APPROVED", "REJECTED", "ESCALATED"
	DecidedBy string    `json:"decided_by"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`

	// UserContext is optional human-provided guidance that gets injected into
	// the next step's context. Useful when the human approves but wants to
	// add domain knowledge the LLM missed.
	// Pattern: user_input from Temporal reinsurance case study.
	UserContext string `json:"user_context,omitempty"`
}

// UserContextSignal allows a human to inject additional context into a running
// workflow without being at a HITL gate. The workflow listens on a separate
// signal channel and merges the context into the next activity's payload.
// Pattern: user interjection from Temporal reinsurance case study.
type UserContextSignal struct {
	// Context is the human-provided guidance or domain knowledge.
	Context string `json:"context"`

	// ProvidedBy identifies who injected the context.
	ProvidedBy string `json:"provided_by"`

	// TargetStep optionally directs the context to a specific step.
	// If empty, it applies to the currently executing or next step.
	TargetStep string `json:"target_step,omitempty"`

	Timestamp time.Time `json:"timestamp"`
}
