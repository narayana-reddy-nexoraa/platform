package compliance

import (
	"context"
	"time"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// Severity indicates how critical a compliance violation is.
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
)

// ComplianceViolation describes a single compliance rule violation.
type ComplianceViolation struct {
	Framework sopdomain.ComplianceFramework `json:"framework"`
	Rule      string                        `json:"rule"`
	Severity  Severity                      `json:"severity"`
	Message   string                        `json:"message"`
	StepID    string                        `json:"step_id,omitempty"`
}

// ComplianceReport is the outcome of running compliance checks against an SOP execution.
type ComplianceReport struct {
	Violations []ComplianceViolation `json:"violations"`
	Passed     bool                  `json:"passed"`
	CheckedAt  time.Time             `json:"checked_at"`
}

// FrameworkValidator is the interface each framework-specific validator implements.
type FrameworkValidator interface {
	Validate(auditEntries []sopdomain.AuditEntry, hitlRequests []sopdomain.HITLRequest) []ComplianceViolation
}

// ComplianceValidator is the top-level validator that routes to framework-specific validators.
type ComplianceValidator struct {
	validators map[sopdomain.ComplianceFramework]FrameworkValidator
}

// NewComplianceValidator constructs a ComplianceValidator with all supported framework validators.
func NewComplianceValidator() *ComplianceValidator {
	v := &ComplianceValidator{
		validators: make(map[sopdomain.ComplianceFramework]FrameworkValidator),
	}
	v.validators[sopdomain.ComplianceHIPAA] = NewHIPAAValidator()
	v.validators[sopdomain.ComplianceCFR21Part11] = NewCFR21Validator()
	v.validators[sopdomain.ComplianceBSAAML] = NewBSAAMLValidator()
	v.validators[sopdomain.ComplianceSOX] = NewSOXValidator()
	return v
}

// Validate runs compliance checks for the given SOP execution against all requested frameworks.
// It routes each framework tag to its dedicated validator and aggregates the results.
func (cv *ComplianceValidator) Validate(
	ctx context.Context,
	sopExecution sopdomain.SOPExecution,
	auditEntries []sopdomain.AuditEntry,
	complianceFrameworks []sopdomain.ComplianceFramework,
) ComplianceReport {
	var allViolations []ComplianceViolation

	// Collect HITL requests from audit context (passed externally in real usage).
	// For the validator, we accept audit entries and route based on frameworks.
	// HITL requests are extracted separately; here we pass an empty slice
	// and rely on the overload Validate method for framework validators.
	// In practice, callers should use ValidateWithHITL for full checking.

	for _, fw := range complianceFrameworks {
		fwValidator, ok := cv.validators[fw]
		if !ok {
			allViolations = append(allViolations, ComplianceViolation{
				Framework: fw,
				Rule:      "UNSUPPORTED_FRAMEWORK",
				Severity:  SeverityWarning,
				Message:   "no validator registered for framework: " + string(fw),
			})
			continue
		}
		violations := fwValidator.Validate(auditEntries, nil)
		allViolations = append(allViolations, violations...)
	}

	passed := true
	for _, v := range allViolations {
		if v.Severity == SeverityError {
			passed = false
			break
		}
	}

	return ComplianceReport{
		Violations: allViolations,
		Passed:     passed,
		CheckedAt:  time.Now(),
	}
}

// ValidateWithHITL runs compliance checks including HITL request validation.
func (cv *ComplianceValidator) ValidateWithHITL(
	ctx context.Context,
	sopExecution sopdomain.SOPExecution,
	auditEntries []sopdomain.AuditEntry,
	hitlRequests []sopdomain.HITLRequest,
	complianceFrameworks []sopdomain.ComplianceFramework,
) ComplianceReport {
	var allViolations []ComplianceViolation

	for _, fw := range complianceFrameworks {
		fwValidator, ok := cv.validators[fw]
		if !ok {
			allViolations = append(allViolations, ComplianceViolation{
				Framework: fw,
				Rule:      "UNSUPPORTED_FRAMEWORK",
				Severity:  SeverityWarning,
				Message:   "no validator registered for framework: " + string(fw),
			})
			continue
		}
		violations := fwValidator.Validate(auditEntries, hitlRequests)
		allViolations = append(allViolations, violations...)
	}

	passed := true
	for _, v := range allViolations {
		if v.Severity == SeverityError {
			passed = false
			break
		}
	}

	return ComplianceReport{
		Violations: allViolations,
		Passed:     passed,
		CheckedAt:  time.Now(),
	}
}
