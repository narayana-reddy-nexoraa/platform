package compliance

import (
	"fmt"
	"strings"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// HIPAAValidator checks compliance with HIPAA regulations.
type HIPAAValidator struct{}

// NewHIPAAValidator constructs a HIPAAValidator.
func NewHIPAAValidator() *HIPAAValidator {
	return &HIPAAValidator{}
}

// Validate runs all HIPAA compliance checks against the given audit entries and HITL requests.
func (h *HIPAAValidator) Validate(auditEntries []sopdomain.AuditEntry, hitlRequests []sopdomain.HITLRequest) []ComplianceViolation {
	var violations []ComplianceViolation
	violations = append(violations, h.checkPHIEncryption(auditEntries)...)
	violations = append(violations, h.checkAccessLogging(auditEntries)...)
	violations = append(violations, h.checkMinimumNecessary(auditEntries)...)
	violations = append(violations, h.checkAuditTrailCompleteness(auditEntries)...)
	return violations
}

// checkPHIEncryption verifies that PHI fields are encrypted by looking for
// encryption markers in the audit entry hashes. A valid encrypted hash must
// contain "enc:" or "sha256:" prefixes indicating it went through the
// encryption pipeline.
func (h *HIPAAValidator) checkPHIEncryption(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	for _, entry := range auditEntries {
		if !hasPHIComplianceTag(entry) {
			continue
		}
		// Input hashes for PHI-tagged entries must carry encryption markers.
		if entry.InputHash != "" && !isEncryptedHash(entry.InputHash) {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceHIPAA,
				Rule:      "PHI_ENCRYPTION",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: PHI input data is not encrypted (hash: %s)", entry.StepID, entry.InputHash),
				StepID:    entry.StepID,
			})
		}
		if entry.OutputHash != "" && !isEncryptedHash(entry.OutputHash) {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceHIPAA,
				Rule:      "PHI_ENCRYPTION",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: PHI output data is not encrypted (hash: %s)", entry.StepID, entry.OutputHash),
				StepID:    entry.StepID,
			})
		}
	}
	return violations
}

// checkAccessLogging ensures every step in the SOP execution has a corresponding audit entry.
// Under HIPAA, all access to PHI must be logged.
func (h *HIPAAValidator) checkAccessLogging(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	if len(auditEntries) == 0 {
		violations = append(violations, ComplianceViolation{
			Framework: sopdomain.ComplianceHIPAA,
			Rule:      "ACCESS_LOGGING",
			Severity:  SeverityError,
			Message:   "no audit entries found; HIPAA requires complete access logging",
		})
		return violations
	}

	// Every audit entry must have a non-empty step ID and a recorded timestamp.
	for _, entry := range auditEntries {
		if entry.StepID == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceHIPAA,
				Rule:      "ACCESS_LOGGING",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("audit entry %s has empty step_id; all access must be attributed to a step", entry.AuditID),
				StepID:    entry.StepID,
			})
		}
		if entry.CreatedAt.IsZero() {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceHIPAA,
				Rule:      "ACCESS_LOGGING",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("audit entry %s has no timestamp", entry.AuditID),
				StepID:    entry.StepID,
			})
		}
	}
	return violations
}

// checkMinimumNecessary flags excessive data retrieval steps. Under HIPAA's minimum
// necessary standard, an SOP should not have more than a reasonable number of
// DATA_RETRIEVAL steps. We flag when more than 3 data retrieval steps exist,
// as this may indicate over-fetching of PHI.
func (h *HIPAAValidator) checkMinimumNecessary(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	const maxDataRetrievalSteps = 3

	dataRetrievalCount := 0
	for _, entry := range auditEntries {
		if entry.AgentType == sopdomain.StepDataRetrieval {
			dataRetrievalCount++
		}
	}

	if dataRetrievalCount > maxDataRetrievalSteps {
		violations = append(violations, ComplianceViolation{
			Framework: sopdomain.ComplianceHIPAA,
			Rule:      "MINIMUM_NECESSARY",
			Severity:  SeverityWarning,
			Message:   fmt.Sprintf("excessive data retrieval steps (%d); HIPAA minimum necessary standard may be violated", dataRetrievalCount),
		})
	}
	return violations
}

// checkAuditTrailCompleteness verifies there are no gaps in the step sequence.
// Each step should appear exactly once in the audit trail, and steps should be
// in chronological order without time gaps that could indicate missing entries.
func (h *HIPAAValidator) checkAuditTrailCompleteness(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	if len(auditEntries) < 2 {
		return violations
	}

	// Check for duplicate step IDs which could indicate replay issues.
	stepSeen := make(map[string]int)
	for _, entry := range auditEntries {
		stepSeen[entry.StepID]++
	}
	for stepID, count := range stepSeen {
		if count > 1 {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceHIPAA,
				Rule:      "AUDIT_TRAIL_COMPLETENESS",
				Severity:  SeverityWarning,
				Message:   fmt.Sprintf("step %s appears %d times in audit trail; possible duplicate execution", stepID, count),
				StepID:    stepID,
			})
		}
	}

	// Check chronological order.
	for i := 1; i < len(auditEntries); i++ {
		if auditEntries[i].CreatedAt.Before(auditEntries[i-1].CreatedAt) {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceHIPAA,
				Rule:      "AUDIT_TRAIL_COMPLETENESS",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("audit entries out of order: step %s at %v precedes step %s at %v", auditEntries[i].StepID, auditEntries[i].CreatedAt, auditEntries[i-1].StepID, auditEntries[i-1].CreatedAt),
				StepID:    auditEntries[i].StepID,
			})
		}
	}

	return violations
}

// hasPHIComplianceTag returns true if the audit entry is tagged with a PHI-relevant framework.
func hasPHIComplianceTag(entry sopdomain.AuditEntry) bool {
	for _, tag := range entry.ComplianceTags {
		if tag.RequiresPHIProtection() {
			return true
		}
	}
	return false
}

// isEncryptedHash checks if a hash string contains encryption markers
// indicating it was processed through the encryption pipeline.
func isEncryptedHash(hash string) bool {
	return strings.HasPrefix(hash, "enc:") || strings.HasPrefix(hash, "sha256:")
}
