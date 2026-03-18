package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditEntry records a single agent action within an SOP execution.
type AuditEntry struct {
	AuditID          uuid.UUID             `json:"audit_id"`
	SOPExecutionID   uuid.UUID             `json:"sop_execution_id"`
	SOPID            string                `json:"sop_id"`
	TenantID         uuid.UUID             `json:"tenant_id"`
	StepID           string                `json:"step_id"`
	AgentType        StepType              `json:"agent_type"`
	Action           string                `json:"action"`
	InputHash        string                `json:"input_hash"`
	OutputHash       string                `json:"output_hash"`
	ModelUsed        *string               `json:"model_used,omitempty"`
	LatencyMs        int64                 `json:"latency_ms"`
	TokensUsed       *int32                `json:"tokens_used,omitempty"`
	ComplianceTags   []ComplianceFramework `json:"compliance_tags"`
	CreatedAt        time.Time             `json:"created_at"`
}

// AuditEntryResponse is the API output for an audit entry.
type AuditEntryResponse struct {
	AuditID          uuid.UUID             `json:"audit_id"`
	SOPExecutionID   uuid.UUID             `json:"sop_execution_id"`
	SOPID            string                `json:"sop_id"`
	StepID           string                `json:"step_id"`
	AgentType        StepType              `json:"agent_type"`
	Action           string                `json:"action"`
	InputHash        string                `json:"input_hash"`
	OutputHash       string                `json:"output_hash"`
	ModelUsed        *string               `json:"model_used,omitempty"`
	LatencyMs        int64                 `json:"latency_ms"`
	TokensUsed       *int32                `json:"tokens_used,omitempty"`
	ComplianceTags   []ComplianceFramework `json:"compliance_tags"`
	CreatedAt        time.Time             `json:"created_at"`
}

// ToResponse converts a domain AuditEntry to an API response.
func (a *AuditEntry) ToResponse() AuditEntryResponse {
	return AuditEntryResponse{
		AuditID:        a.AuditID,
		SOPExecutionID: a.SOPExecutionID,
		SOPID:          a.SOPID,
		StepID:         a.StepID,
		AgentType:      a.AgentType,
		Action:         a.Action,
		InputHash:      a.InputHash,
		OutputHash:     a.OutputHash,
		ModelUsed:      a.ModelUsed,
		LatencyMs:      a.LatencyMs,
		TokensUsed:     a.TokensUsed,
		ComplianceTags: a.ComplianceTags,
		CreatedAt:      a.CreatedAt,
	}
}

// AuditTrailResponse wraps the full audit trail for an SOP execution.
type AuditTrailResponse struct {
	SOPExecutionID uuid.UUID            `json:"sop_execution_id"`
	SOPID          string               `json:"sop_id"`
	TenantID       uuid.UUID            `json:"tenant_id"`
	Entries        []AuditEntryResponse `json:"entries"`
	TotalEntries   int                  `json:"total_entries"`
}
