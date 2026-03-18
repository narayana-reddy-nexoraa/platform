package domain

import (
	"time"

	"github.com/google/uuid"
)

// HITLDecision represents the human reviewer's decision.
type HITLDecision string

const (
	HITLPending   HITLDecision = "PENDING"
	HITLApproved  HITLDecision = "APPROVED"
	HITLRejected  HITLDecision = "REJECTED"
	HITLEscalated HITLDecision = "ESCALATED"
)

// HITLRequest is a human-in-the-loop approval request generated during SOP execution.
type HITLRequest struct {
	RequestID       uuid.UUID    `json:"request_id"`
	SOPExecutionID  uuid.UUID    `json:"sop_execution_id"`
	SOPID           string       `json:"sop_id"`
	TenantID        uuid.UUID    `json:"tenant_id"`
	StepID          string       `json:"step_id"`
	StepName        string       `json:"step_name"`
	Decision        HITLDecision `json:"decision"`
	DecidedBy       *string      `json:"decided_by,omitempty"`
	DecisionReason  *string      `json:"decision_reason,omitempty"`
	DecidedAt       *time.Time   `json:"decided_at,omitempty"`
	Deadline        time.Time    `json:"deadline"`
	Payload         []byte       `json:"payload"`
	TemporalWorkflowID string   `json:"temporal_workflow_id"`
	TemporalRunID   string       `json:"temporal_run_id"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	Version         int32        `json:"version"`
}

// HITLRequestResponse is the API output for an HITL request.
type HITLRequestResponse struct {
	RequestID       uuid.UUID    `json:"request_id"`
	SOPExecutionID  uuid.UUID    `json:"sop_execution_id"`
	SOPID           string       `json:"sop_id"`
	TenantID        uuid.UUID    `json:"tenant_id"`
	StepID          string       `json:"step_id"`
	StepName        string       `json:"step_name"`
	Decision        HITLDecision `json:"decision"`
	DecidedBy       *string      `json:"decided_by,omitempty"`
	DecisionReason  *string      `json:"decision_reason,omitempty"`
	DecidedAt       *time.Time   `json:"decided_at,omitempty"`
	Deadline        time.Time    `json:"deadline"`
	CreatedAt       time.Time    `json:"created_at"`
	IsOverdue       bool         `json:"is_overdue"`
}

// ToResponse converts a domain HITLRequest to an API response.
func (r *HITLRequest) ToResponse() HITLRequestResponse {
	return HITLRequestResponse{
		RequestID:      r.RequestID,
		SOPExecutionID: r.SOPExecutionID,
		SOPID:          r.SOPID,
		TenantID:       r.TenantID,
		StepID:         r.StepID,
		StepName:       r.StepName,
		Decision:       r.Decision,
		DecidedBy:      r.DecidedBy,
		DecisionReason: r.DecisionReason,
		DecidedAt:      r.DecidedAt,
		Deadline:       r.Deadline,
		CreatedAt:      r.CreatedAt,
		IsOverdue:      r.Decision == HITLPending && time.Now().After(r.Deadline),
	}
}

// HITLDecideRequest is the input for approving/rejecting an HITL request.
type HITLDecideRequest struct {
	Decision HITLDecision `json:"decision" binding:"required"`
	Reason   string       `json:"reason"`
	DecidedBy string     `json:"decided_by" binding:"required"`
}

// HITLApprovalSignal is the Temporal signal payload for HITL approval.
type HITLApprovalSignal struct {
	Decision  HITLDecision `json:"decision"`
	DecidedBy string       `json:"decided_by"`
	Reason    string       `json:"reason"`
	Timestamp time.Time    `json:"timestamp"`
}
