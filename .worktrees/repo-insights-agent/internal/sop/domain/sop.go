package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SOPDefinition is the blueprint for a Standard Operating Procedure.
// Each SOP is a compile-time Go struct, versioned in source control.
type SOPDefinition struct {
	// SOPID uniquely identifies the SOP (e.g., "FS-01", "INS-01", "HC-01").
	SOPID string `json:"sop_id"`

	// Name is the human-readable SOP name.
	Name string `json:"name"`

	// Industry is the business vertical this SOP serves.
	Industry Industry `json:"industry"`

	// Version tracks the SOP definition version (semver-style).
	Version string `json:"version"`

	// Description explains what this SOP automates.
	Description string `json:"description"`

	// Steps is the ordered list of agent steps (typically 6: intake→data→classify→decide→execute→audit).
	Steps []AgentStep `json:"steps"`

	// ComplianceFrameworks lists regulations this SOP must satisfy.
	ComplianceFrameworks []ComplianceFramework `json:"compliance_frameworks"`

	// ProcessOwner is the business role responsible (e.g., "Chief Compliance Officer").
	ProcessOwner string `json:"process_owner"`

	// PrimaryUsers lists roles that interact with this SOP.
	PrimaryUsers []string `json:"primary_users"`

	// VolumeEstimate describes expected throughput (e.g., "500-5,000 cases/month").
	VolumeEstimate string `json:"volume_estimate"`
}

// RequiresHITL returns true if any step in this SOP has a human-in-the-loop gate.
func (s *SOPDefinition) RequiresHITL() bool {
	for _, step := range s.Steps {
		if step.HITLRequired {
			return true
		}
	}
	return false
}

// HITLSteps returns only the steps that require human approval.
func (s *SOPDefinition) HITLSteps() []AgentStep {
	var hitlSteps []AgentStep
	for _, step := range s.Steps {
		if step.HITLRequired {
			hitlSteps = append(hitlSteps, step)
		}
	}
	return hitlSteps
}

// TaskQueue returns the Temporal task queue for this SOP based on its industry.
func (s *SOPDefinition) TaskQueue() string {
	return s.Industry.TaskQueue()
}

// MaxRetentionYears returns the highest retention requirement across all compliance frameworks.
func (s *SOPDefinition) MaxRetentionYears() int {
	maxYears := 5
	for _, cf := range s.ComplianceFrameworks {
		if years := cf.RetentionYears(); years > maxYears {
			maxYears = years
		}
	}
	return maxYears
}

// --- SOP Execution (runtime instance of an SOP) ---

// SOPExecutionStatus tracks the lifecycle of an SOP execution.
type SOPExecutionStatus string

const (
	SOPStatusPending    SOPExecutionStatus = "PENDING"
	SOPStatusRunning    SOPExecutionStatus = "RUNNING"
	SOPStatusWaitingHITL SOPExecutionStatus = "WAITING_HITL"
	SOPStatusCompleted  SOPExecutionStatus = "COMPLETED"
	SOPStatusFailed     SOPExecutionStatus = "FAILED"
	SOPStatusCanceled   SOPExecutionStatus = "CANCELED"
	SOPStatusEscalated  SOPExecutionStatus = "ESCALATED"
)

// SOPExecution is a runtime instance of an SOP being processed.
type SOPExecution struct {
	SOPExecutionID     uuid.UUID          `json:"sop_execution_id"`
	SOPID              string             `json:"sop_id"`
	TenantID           uuid.UUID          `json:"tenant_id"`
	Industry           Industry           `json:"industry"`
	CurrentStep        string             `json:"current_step"`
	Status             SOPExecutionStatus `json:"status"`
	InputPayload       json.RawMessage    `json:"input_payload"`
	OutputPayload      json.RawMessage    `json:"output_payload,omitempty"`
	TemporalWorkflowID string            `json:"temporal_workflow_id,omitempty"`
	TemporalRunID      string             `json:"temporal_run_id,omitempty"`
	StartedAt          time.Time          `json:"started_at"`
	CompletedAt        *time.Time         `json:"completed_at,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	Version            int32              `json:"version"`
}

// SOPExecutionResponse is the API output for an SOP execution.
type SOPExecutionResponse struct {
	SOPExecutionID     uuid.UUID          `json:"sop_execution_id"`
	SOPID              string             `json:"sop_id"`
	TenantID           uuid.UUID          `json:"tenant_id"`
	Industry           Industry           `json:"industry"`
	CurrentStep        string             `json:"current_step"`
	Status             SOPExecutionStatus `json:"status"`
	InputPayload       json.RawMessage    `json:"input_payload"`
	OutputPayload      json.RawMessage    `json:"output_payload,omitempty"`
	TemporalWorkflowID string            `json:"temporal_workflow_id,omitempty"`
	StartedAt          time.Time          `json:"started_at"`
	CompletedAt        *time.Time         `json:"completed_at,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	Version            int32              `json:"version"`
}

// ToResponse converts a domain SOPExecution to an API response.
func (e *SOPExecution) ToResponse() SOPExecutionResponse {
	return SOPExecutionResponse{
		SOPExecutionID:     e.SOPExecutionID,
		SOPID:              e.SOPID,
		TenantID:           e.TenantID,
		Industry:           e.Industry,
		CurrentStep:        e.CurrentStep,
		Status:             e.Status,
		InputPayload:       e.InputPayload,
		OutputPayload:      e.OutputPayload,
		TemporalWorkflowID: e.TemporalWorkflowID,
		StartedAt:          e.StartedAt,
		CompletedAt:        e.CompletedAt,
		CreatedAt:          e.CreatedAt,
		UpdatedAt:          e.UpdatedAt,
		Version:            e.Version,
	}
}

// SOPPaginatedResponse wraps a list response with pagination metadata.
type SOPPaginatedResponse struct {
	Data       []SOPExecutionResponse `json:"data"`
	TotalCount int64                  `json:"total_count"`
	Limit      int32                  `json:"limit"`
	Offset     int32                  `json:"offset"`
}

// StartSOPExecutionRequest is the input for starting a new SOP execution.
type StartSOPExecutionRequest struct {
	Payload json.RawMessage `json:"payload" binding:"required"`
}

// TemporalWorkflowID generates a deterministic workflow ID for Temporal.
func TemporalWorkflowID(sopID string, executionID uuid.UUID) string {
	return fmt.Sprintf("sop-%s-%s", sopID, executionID.String())
}
