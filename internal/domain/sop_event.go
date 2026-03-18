package domain

// SOP aggregate type constants for outbox events.
const (
	AggregateExecution    = "execution"      // Generic execution engine (existing)
	AggregateSOPExecution = "sop_execution"  // SOP lifecycle events
	AggregateHITLRequest  = "hitl_request"   // HITL approval requests
	AggregateHITLResponse = "hitl_response"  // HITL decisions
	AggregateAuditTrail   = "audit_trail"    // Compliance audit events
)

// SOP event type constants for outbox events.
const (
	// SOP execution lifecycle
	EventSOPStarted       = "sop.execution.started"
	EventSOPStepCompleted = "sop.execution.step_completed"
	EventSOPCompleted     = "sop.execution.completed"
	EventSOPFailed        = "sop.execution.failed"
	EventSOPEscalated     = "sop.execution.escalated"
	EventSOPCanceled      = "sop.execution.canceled"

	// HITL lifecycle
	EventHITLRequestCreated  = "hitl.request.created"
	EventHITLRequestExpired  = "hitl.request.expired"
	EventHITLResponseDecided = "hitl.response.decided"

	// Audit
	EventAuditEntryCreated = "audit.entry.created"
)
