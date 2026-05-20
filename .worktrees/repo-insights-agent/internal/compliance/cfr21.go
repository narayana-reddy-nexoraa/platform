package compliance

import (
	"fmt"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// CFR21Validator checks compliance with 21 CFR Part 11 (electronic records and signatures).
type CFR21Validator struct{}

// NewCFR21Validator constructs a CFR21Validator.
func NewCFR21Validator() *CFR21Validator {
	return &CFR21Validator{}
}

// Validate runs all 21 CFR Part 11 compliance checks.
func (c *CFR21Validator) Validate(auditEntries []sopdomain.AuditEntry, hitlRequests []sopdomain.HITLRequest) []ComplianceViolation {
	var violations []ComplianceViolation
	violations = append(violations, c.checkElectronicSignatures(hitlRequests)...)
	violations = append(violations, c.checkAuditTrailCompleteness(auditEntries)...)
	violations = append(violations, c.checkDataIntegrity(auditEntries)...)
	violations = append(violations, c.checkRecordRetention(auditEntries)...)
	return violations
}

// checkElectronicSignatures ensures that all HITL decisions have a decided_by field.
// 21 CFR Part 11 requires that electronic signatures be attributable to a specific individual.
func (c *CFR21Validator) checkElectronicSignatures(hitlRequests []sopdomain.HITLRequest) []ComplianceViolation {
	var violations []ComplianceViolation
	for _, req := range hitlRequests {
		// Only check decisions that are not pending.
		if req.Decision == sopdomain.HITLPending {
			continue
		}
		if req.DecidedBy == nil || *req.DecidedBy == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceCFR21Part11,
				Rule:      "ELECTRONIC_SIGNATURE",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("HITL request %s (step %s) has decision %s without electronic signature (decided_by is empty)", req.RequestID, req.StepID, req.Decision),
				StepID:    req.StepID,
			})
		}
		if req.DecidedAt == nil {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceCFR21Part11,
				Rule:      "ELECTRONIC_SIGNATURE",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("HITL request %s (step %s) has decision without timestamp", req.RequestID, req.StepID),
				StepID:    req.StepID,
			})
		}
	}
	return violations
}

// checkAuditTrailCompleteness verifies every step is audited with a valid timestamp.
// 21 CFR Part 11 requires a complete, computer-generated, time-stamped audit trail.
func (c *CFR21Validator) checkAuditTrailCompleteness(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	if len(auditEntries) == 0 {
		violations = append(violations, ComplianceViolation{
			Framework: sopdomain.ComplianceCFR21Part11,
			Rule:      "AUDIT_TRAIL",
			Severity:  SeverityError,
			Message:   "no audit entries found; 21 CFR Part 11 requires a complete audit trail",
		})
		return violations
	}

	for _, entry := range auditEntries {
		if entry.StepID == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceCFR21Part11,
				Rule:      "AUDIT_TRAIL",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("audit entry %s has empty step_id", entry.AuditID),
				StepID:    entry.StepID,
			})
		}
		if entry.CreatedAt.IsZero() {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceCFR21Part11,
				Rule:      "AUDIT_TRAIL",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("audit entry %s (step %s) has no timestamp", entry.AuditID, entry.StepID),
				StepID:    entry.StepID,
			})
		}
		if entry.Action == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceCFR21Part11,
				Rule:      "AUDIT_TRAIL",
				Severity:  SeverityWarning,
				Message:   fmt.Sprintf("audit entry %s (step %s) has no action recorded", entry.AuditID, entry.StepID),
				StepID:    entry.StepID,
			})
		}
	}
	return violations
}

// checkDataIntegrity ensures that input/output hashes are present and non-empty for every step.
// 21 CFR Part 11 requires mechanisms to ensure data integrity and detect alterations.
func (c *CFR21Validator) checkDataIntegrity(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	for _, entry := range auditEntries {
		if entry.InputHash == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceCFR21Part11,
				Rule:      "DATA_INTEGRITY",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: input hash is missing; data integrity cannot be verified", entry.StepID),
				StepID:    entry.StepID,
			})
		}
		if entry.OutputHash == "" {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceCFR21Part11,
				Rule:      "DATA_INTEGRITY",
				Severity:  SeverityError,
				Message:   fmt.Sprintf("step %s: output hash is missing; data integrity cannot be verified", entry.StepID),
				StepID:    entry.StepID,
			})
		}
	}
	return violations
}

// checkRecordRetention verifies that audit entries carry compliance tags.
// Under 21 CFR Part 11, electronic records must be tagged for proper retention and retrieval.
func (c *CFR21Validator) checkRecordRetention(auditEntries []sopdomain.AuditEntry) []ComplianceViolation {
	var violations []ComplianceViolation
	for _, entry := range auditEntries {
		if len(entry.ComplianceTags) == 0 {
			violations = append(violations, ComplianceViolation{
				Framework: sopdomain.ComplianceCFR21Part11,
				Rule:      "RECORD_RETENTION",
				Severity:  SeverityWarning,
				Message:   fmt.Sprintf("step %s: audit entry has no compliance tags; record retention policy cannot be enforced", entry.StepID),
				StepID:    entry.StepID,
			})
		}
	}
	return violations
}
