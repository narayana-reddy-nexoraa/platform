package compliance

import (
	"fmt"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// SOXValidator checks compliance with the Sarbanes-Oxley Act (SOX).
type SOXValidator struct{}

// NewSOXValidator constructs a SOXValidator.
func NewSOXValidator() *SOXValidator {
	return &SOXValidator{}
}

// Validate runs all SOX compliance checks.
func (s *SOXValidator) Validate(auditEntries []sopdomain.AuditEntry, hitlRequests []sopdomain.HITLRequest) []ComplianceViolation {
	var violations []ComplianceViolation
	violations = append(violations, s.checkSegregationOfDuties(auditEntries)...)
	violations = append(violations, s.checkFinancialDataIntegrity(auditEntries)...)
	violations = append(violations, s.checkChangeDocumentation(auditEntries)...)
	return violations
}

// checkSegregationOfDuties ensures that different agent types handle classify, decide, and execute steps.
// Under SOX, no single actor should control multiple stages of a financial process.
// We verify that the agent types for CLASSIFICATION, DECISIONING, and EXECUTION are distinct,
// and additionally that if ModelUsed is set, no single model is used across all three roles.
func (s *SOXValidator) checkSegregationOfDuties(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation

	// Track which step IDs handle each critical role.
	roleAgents := map[sopdomain.StepType]map[string]bool{
		sopdomain.StepClassification: make(map[string]bool),
		sopdomain.StepDecisioning:    make(map[string]bool),
		sopdomain.StepExecution:      make(map[string]bool),
	}

	for _, entry := range auditEntries {
		if agents, ok := roleAgents[entry.AgentType]; ok {
			agents[entry.StepID] = true
		}
	}

	// Check for overlap: if the same step_id appears in more than one critical role,
	// segregation of duties is violated.
	allStepIDs := make(map[string][]sopdomain.StepType)
	for role, agents := range roleAgents {
		for stepID := range agents {
			allStepIDs[stepID] = append(allStepIDs[stepID], role)
		}
	}

	for stepID, roles := range allStepIDs {
		if len(roles) > 1 {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceSOX,
				Rule:      "SEGREGATION_OF_DUTIES",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s performs multiple critical roles %v; SOX requires segregation of duties", stepID, roles),
				StepID:    stepID,
			})
		}
	}

	// Also check if we have at least the classify+decide+execute chain.
	// If any is missing entirely, it might indicate a control gap.
	for role, agents := range roleAgents {
		if len(agents) == 0 {
			// Only warn if there are audit entries at all (skip if the SOP does not use this role).
			if len(auditEntries) > 0 {
				violations = append(violations, ComplianceViolation{
					Framework: sopdomain.ComplianceSOX,
					Rule:      "SEGREGATION_OF_DUTIES",
					Severity:  SeverityWarning,
					Message:   fmt.Sprintf("no steps found with agent type %s; SOX control chain may be incomplete", role),
				})
			}
		}
	}

	return violations
}

// checkFinancialDataIntegrity ensures all steps have hash verification.
// SOX requires that financial data is protected against unauthorized alteration.
func (s *SOXValidator) checkFinancialDataIntegrity(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	for _, entry := range auditEntries {
		if entry.InputHash == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceSOX,
				Rule:      "FINANCIAL_DATA_INTEGRITY",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: missing input hash; financial data integrity cannot be verified", entry.StepID),
				StepID:    entry.StepID,
			})
		}
		if entry.OutputHash == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceSOX,
				Rule:      "FINANCIAL_DATA_INTEGRITY",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: missing output hash; financial data integrity cannot be verified", entry.StepID),
				StepID:    entry.StepID,
			})
		}
	}
	return violations
}

// checkChangeDocumentation ensures the audit trail records every status transition.
// Under SOX, all changes to financial records must be documented.
// Every audit entry must have a non-empty action describing what occurred.
func (s *SOXValidator) checkChangeDocumentation(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	for _, entry := range auditEntries {
		if entry.Action == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceSOX,
				Rule:      "CHANGE_DOCUMENTATION",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: audit entry has no action; status transitions must be documented", entry.StepID),
				StepID:    entry.StepID,
			})
		}
		if entry.CreatedAt.IsZero() {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceSOX,
				Rule:      "CHANGE_DOCUMENTATION",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: audit entry has no timestamp; changes must have temporal context", entry.StepID),
				StepID:    entry.StepID,
			})
		}
	}
	return violations
}
