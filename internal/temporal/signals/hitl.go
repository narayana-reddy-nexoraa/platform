package signals

import "time"

// HITLSignalName is the Temporal signal channel name for HITL approval.
const HITLSignalName = "hitl-approval"

// HITLApproval is the signal payload sent by the HITL UI when a human makes a decision.
type HITLApproval struct {
	Decision  string    `json:"decision"`  // "APPROVED", "REJECTED", "ESCALATED"
	DecidedBy string    `json:"decided_by"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}
