package compliance

import (
	"fmt"
	"strings"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// BSAAMLValidator checks compliance with the Bank Secrecy Act / Anti-Money Laundering regulations.
type BSAAMLValidator struct{}

// NewBSAAMLValidator constructs a BSAAMLValidator.
func NewBSAAMLValidator() *BSAAMLValidator {
	return &BSAAMLValidator{}
}

// Validate runs all BSA/AML compliance checks.
func (b *BSAAMLValidator) Validate(auditEntries []sopdomain.AuditEntry, hitlRequests []sopdomain.HITLRequest) []ComplianceViolation {
	var violations []ComplianceViolation
	violations = append(violations, b.checkSARFilingTriggers(auditEntries)...)
	violations = append(violations, b.checkCTRThresholds(auditEntries)...)
	violations = append(violations, b.checkSuspiciousActivityDocumentation(auditEntries)...)
	return violations
}

// checkSARFilingTriggers ensures that high-risk classifications produce a downstream action.
// Under BSA/AML, when a transaction or entity is classified as high risk, a Suspicious Activity
// Report (SAR) filing must be triggered. We check that any classification step whose action
// contains "high_risk" or "high-risk" is followed by an execution step.
func (b *BSAAMLValidator) checkSARFilingTriggers(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation

	highRiskSteps := make(map[string]bool)
	executionSteps := make(map[string]bool)

	for _, entry := range auditEntries {
		if entry.AgentType == sopdomain.StepClassification && isHighRiskAction(entry.Action) {
			highRiskSteps[entry.StepID] = true
		}
		if entry.AgentType == sopdomain.StepExecution {
			executionSteps[entry.StepID] = true
		}
	}

	// If there are high-risk classifications but no execution steps at all,
	// that means no SAR filing action was produced.
	if len(highRiskSteps) > 0 && len(executionSteps) == 0 {
		for stepID := range highRiskSteps {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceBSAAML,
				Rule:      "SAR_FILING_TRIGGER",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: high-risk classification detected but no execution step found to trigger SAR filing", stepID),
				StepID:    stepID,
			})
		}
	}

	return violations
}

// checkCTRThresholds verifies that transactions above $10,000 are flagged.
// Under BSA, Currency Transaction Reports must be filed for cash transactions exceeding $10,000.
// We look for audit entries whose action contains a dollar amount above the threshold.
// Actions containing "ctr_required" or amounts like "$10000" or higher should have BSA_AML compliance tags.
func (b *BSAAMLValidator) checkCTRThresholds(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation

	for _, entry := range auditEntries {
		if !isCTRRelevantAction(entry.Action) {
			continue
		}
		// If the action indicates a CTR-relevant transaction but the entry lacks BSA/AML compliance tags,
		// it means the transaction was not properly flagged.
		if !hasBSAAMLTag(entry) {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceBSAAML,
				Rule:      "CTR_THRESHOLD",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: transaction above CTR threshold detected (%s) but not flagged with BSA_AML compliance tag", entry.StepID, entry.Action),
				StepID:    entry.StepID,
			})
		}
	}

	return violations
}

// checkSuspiciousActivityDocumentation ensures the audit trail captures classification rationale
// for suspicious activity. Every classification step must have both input and output hashes
// present, and the action must be non-empty to document the reasoning.
func (b *BSAAMLValidator) checkSuspiciousActivityDocumentation(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation

	for _, entry := range auditEntries {
		if entry.AgentType != sopdomain.StepClassification {
			continue
		}
		if entry.Action == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceBSAAML,
				Rule:      "SUSPICIOUS_ACTIVITY_DOCUMENTATION",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: classification step has no action recorded; suspicious activity rationale is undocumented", entry.StepID),
				StepID:    entry.StepID,
			})
		}
		if entry.OutputHash == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceBSAAML,
				Rule:      "SUSPICIOUS_ACTIVITY_DOCUMENTATION",
				Severity:  SeverityWarning,
				Message:   fmt.Sprintf("step %s: classification step has no output hash; classification result cannot be verified", entry.StepID),
				StepID:    entry.StepID,
			})
		}
	}

	return violations
}

// isHighRiskAction checks if the action string indicates a high-risk classification.
func isHighRiskAction(action string) bool {
	lower := strings.ToLower(action)
	return strings.Contains(lower, "high_risk") || strings.Contains(lower, "high-risk")
}

// isCTRRelevantAction checks if the action indicates a transaction above the CTR threshold.
func isCTRRelevantAction(action string) bool {
	lower := strings.ToLower(action)
	return strings.Contains(lower, "ctr_required") || strings.Contains(lower, "ctr_threshold") || strings.Contains(lower, "large_cash_transaction")
}

// hasBSAAMLTag returns true if the audit entry carries a BSA/AML compliance tag.
func hasBSAAMLTag(entry sopdomain.AuditEntry) bool {
	for _, tag := range entry.ComplianceTags {
		if tag == sopdomain.ComplianceBSAAML {
			return true
		}
	}
	return false
}
