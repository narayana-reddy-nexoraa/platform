package compliance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newAuditEntry(stepID string, agentType sopdomain.StepType, action, inputHash, outputHash string, tags []sopdomain.ComplianceFramework) sopdomain.AuditEntry {
	return sopdomain.AuditEntry{
		AuditID:        uuid.New(),
		SOPExecutionID: uuid.New(),
		SOPID:          "TEST-01",
		TenantID:       uuid.New(),
		StepID:         stepID,
		AgentType:      agentType,
		Action:         action,
		InputHash:      inputHash,
		OutputHash:     outputHash,
		ComplianceTags: tags,
		CreatedAt:      time.Now(),
	}
}

func newHITLRequest(stepID string, decision sopdomain.HITLDecision, decidedBy *string, decidedAt *time.Time) sopdomain.HITLRequest {
	return sopdomain.HITLRequest{
		RequestID:      uuid.New(),
		SOPExecutionID: uuid.New(),
		SOPID:          "TEST-01",
		TenantID:       uuid.New(),
		StepID:         stepID,
		StepName:       "Test Step " + stepID,
		Decision:       decision,
		DecidedBy:      decidedBy,
		DecidedAt:      decidedAt,
		Deadline:       time.Now().Add(1 * time.Hour),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func strPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// ---------------------------------------------------------------------------
// HIPAA Tests
// ---------------------------------------------------------------------------

func TestHIPAAValidator_CatchesMissingAuditEntries(t *testing.T) {
	v := NewHIPAAValidator()

	// Empty audit entries should trigger ACCESS_LOGGING violation.
	violations := v.Validate(nil, nil)
	require.NotEmpty(t, violations)

	found := false
	for _, viol := range violations {
		if viol.Rule == "ACCESS_LOGGING" && viol.Severity == SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ACCESS_LOGGING error for empty audit entries")
}

func TestHIPAAValidator_PHIEncryption_UnencryptedInput(t *testing.T) {
	v := NewHIPAAValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepDataRetrieval, "fetch_patient_records",
			"plaintext_hash_abc123",                         // not encrypted
			"enc:sha256:output_hash",                        // encrypted
			[]sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA}),
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "PHI_ENCRYPTION" && viol.StepID == "step-1" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected PHI_ENCRYPTION violation for unencrypted input hash")
}

func TestHIPAAValidator_PHIEncryption_EncryptedPasses(t *testing.T) {
	v := NewHIPAAValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepDataRetrieval, "fetch_patient_records",
			"enc:aes256:input_hash",
			"sha256:output_hash",
			[]sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA}),
	}

	violations := v.Validate(entries, nil)

	for _, viol := range violations {
		assert.NotEqual(t, "PHI_ENCRYPTION", viol.Rule, "should not flag properly encrypted hashes")
	}
}

func TestHIPAAValidator_MinimumNecessary(t *testing.T) {
	v := NewHIPAAValidator()

	// Create 5 data retrieval steps (exceeds threshold of 3).
	var entries []sopdomain.AuditEntry
	for i := 0; i < 5; i++ {
		entries = append(entries, newAuditEntry(
			"data-step-"+string(rune('a'+i)),
			sopdomain.StepDataRetrieval, "fetch_data",
			"sha256:in", "sha256:out", nil))
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "MINIMUM_NECESSARY" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected MINIMUM_NECESSARY warning for excessive data retrieval steps")
}

func TestHIPAAValidator_AuditTrailCompleteness_OutOfOrder(t *testing.T) {
	v := NewHIPAAValidator()

	now := time.Now()
	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "intake", "sha256:in", "sha256:out", nil),
		newAuditEntry("step-2", sopdomain.StepClassification, "classify", "sha256:in", "sha256:out", nil),
	}
	// Make step-2 have an earlier timestamp than step-1 to break chronological order.
	entries[0].CreatedAt = now
	entries[1].CreatedAt = now.Add(-1 * time.Hour)

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "AUDIT_TRAIL_COMPLETENESS" && viol.Severity == SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected AUDIT_TRAIL_COMPLETENESS error for out-of-order entries")
}

// ---------------------------------------------------------------------------
// 21 CFR Part 11 Tests
// ---------------------------------------------------------------------------

func TestCFR21Validator_CatchesMissingElectronicSignatures(t *testing.T) {
	v := NewCFR21Validator()

	hitlRequests := []sopdomain.HITLRequest{
		newHITLRequest("decide-step", sopdomain.HITLApproved, nil, timePtr(time.Now())), // no decided_by
	}

	violations := v.Validate(nil, hitlRequests)

	found := false
	for _, viol := range violations {
		if viol.Rule == "ELECTRONIC_SIGNATURE" && viol.Severity == SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ELECTRONIC_SIGNATURE error for HITL decision without decided_by")
}

func TestCFR21Validator_CatchesMissingDecisionTimestamp(t *testing.T) {
	v := NewCFR21Validator()

	hitlRequests := []sopdomain.HITLRequest{
		newHITLRequest("decide-step", sopdomain.HITLRejected, strPtr("dr.smith"), nil), // no decided_at
	}

	violations := v.Validate(nil, hitlRequests)

	found := false
	for _, viol := range violations {
		if viol.Rule == "ELECTRONIC_SIGNATURE" && viol.StepID == "decide-step" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ELECTRONIC_SIGNATURE error for HITL decision without timestamp")
}

func TestCFR21Validator_ValidSignaturePasses(t *testing.T) {
	v := NewCFR21Validator()

	now := time.Now()
	hitlRequests := []sopdomain.HITLRequest{
		newHITLRequest("decide-step", sopdomain.HITLApproved, strPtr("dr.smith"), &now),
	}

	violations := v.Validate(nil, hitlRequests)

	for _, viol := range violations {
		assert.NotEqual(t, "ELECTRONIC_SIGNATURE", viol.Rule, "valid signature should not produce e-sig violations")
	}
}

func TestCFR21Validator_DataIntegrity_MissingHashes(t *testing.T) {
	v := NewCFR21Validator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "intake", "", "", nil), // missing both hashes
	}

	violations := v.Validate(entries, nil)

	inputViol, outputViol := false, false
	for _, viol := range violations {
		if viol.Rule == "DATA_INTEGRITY" && viol.StepID == "step-1" {
			if viol.Message != "" {
				if contains(viol.Message, "input hash") {
					inputViol = true
				}
				if contains(viol.Message, "output hash") {
					outputViol = true
				}
			}
		}
	}
	assert.True(t, inputViol, "expected DATA_INTEGRITY violation for missing input hash")
	assert.True(t, outputViol, "expected DATA_INTEGRITY violation for missing output hash")
}

func TestCFR21Validator_RecordRetention_MissingTags(t *testing.T) {
	v := NewCFR21Validator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "intake", "sha256:in", "sha256:out", nil), // no compliance tags
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "RECORD_RETENTION" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected RECORD_RETENTION warning for missing compliance tags")
}

// ---------------------------------------------------------------------------
// BSA/AML Tests
// ---------------------------------------------------------------------------

func TestBSAAMLValidator_CatchesUnflaggedHighRisk(t *testing.T) {
	v := NewBSAAMLValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("classify-step", sopdomain.StepClassification, "classification: HIGH_RISK",
			"sha256:in", "sha256:out", nil),
		// No execution step follows.
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "SAR_FILING_TRIGGER" && viol.Severity == SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected SAR_FILING_TRIGGER error when high-risk classification has no execution step")
}

func TestBSAAMLValidator_HighRiskWithExecutionPasses(t *testing.T) {
	v := NewBSAAMLValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("classify-step", sopdomain.StepClassification, "classification: HIGH_RISK",
			"sha256:in", "sha256:out", nil),
		newAuditEntry("execute-step", sopdomain.StepExecution, "file_sar",
			"sha256:in", "sha256:out", nil),
	}

	violations := v.Validate(entries, nil)

	for _, viol := range violations {
		assert.NotEqual(t, "SAR_FILING_TRIGGER", viol.Rule, "should not trigger SAR violation when execution step present")
	}
}

func TestBSAAMLValidator_CTRThreshold_NotFlagged(t *testing.T) {
	v := NewBSAAMLValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepClassification, "ctr_required: $15000",
			"sha256:in", "sha256:out", nil), // missing BSA_AML tag
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "CTR_THRESHOLD" && viol.Severity == SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected CTR_THRESHOLD error when CTR-relevant transaction lacks BSA_AML tag")
}

func TestBSAAMLValidator_CTRThreshold_ProperlyFlagged(t *testing.T) {
	v := NewBSAAMLValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepClassification, "ctr_required: $15000",
			"sha256:in", "sha256:out",
			[]sopdomain.ComplianceFramework{sopdomain.ComplianceBSAAML}),
	}

	violations := v.Validate(entries, nil)

	for _, viol := range violations {
		assert.NotEqual(t, "CTR_THRESHOLD", viol.Rule, "should not flag when CTR transaction has BSA_AML tag")
	}
}

func TestBSAAMLValidator_SuspiciousActivityDocumentation_MissingAction(t *testing.T) {
	v := NewBSAAMLValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("classify-step", sopdomain.StepClassification, "",
			"sha256:in", "sha256:out", nil), // empty action
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "SUSPICIOUS_ACTIVITY_DOCUMENTATION" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected SUSPICIOUS_ACTIVITY_DOCUMENTATION violation for missing classification action")
}

// ---------------------------------------------------------------------------
// SOX Tests
// ---------------------------------------------------------------------------

func TestSOXValidator_CatchesSameAgentViolation(t *testing.T) {
	v := NewSOXValidator()

	// Same step_id performing both classification and decisioning.
	entries := []sopdomain.AuditEntry{
		newAuditEntry("multi-role-step", sopdomain.StepClassification, "classify",
			"sha256:in", "sha256:out", nil),
		newAuditEntry("multi-role-step", sopdomain.StepDecisioning, "decide",
			"sha256:in", "sha256:out", nil),
		newAuditEntry("execute-step", sopdomain.StepExecution, "execute",
			"sha256:in", "sha256:out", nil),
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "SEGREGATION_OF_DUTIES" && viol.Severity == SeverityError && viol.StepID == "multi-role-step" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected SEGREGATION_OF_DUTIES error when same step performs classify and decide")
}

func TestSOXValidator_ProperSegregationPasses(t *testing.T) {
	v := NewSOXValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("classify-step", sopdomain.StepClassification, "classify",
			"sha256:in", "sha256:out", nil),
		newAuditEntry("decide-step", sopdomain.StepDecisioning, "decide",
			"sha256:in", "sha256:out", nil),
		newAuditEntry("execute-step", sopdomain.StepExecution, "execute",
			"sha256:in", "sha256:out", nil),
	}

	violations := v.Validate(entries, nil)

	for _, viol := range violations {
		if viol.Rule == "SEGREGATION_OF_DUTIES" && viol.Severity == SeverityError {
			t.Errorf("unexpected SEGREGATION_OF_DUTIES error: %s", viol.Message)
		}
	}
}

func TestSOXValidator_FinancialDataIntegrity_MissingHashes(t *testing.T) {
	v := NewSOXValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "intake", "", "sha256:out", nil),
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "FINANCIAL_DATA_INTEGRITY" && viol.StepID == "step-1" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected FINANCIAL_DATA_INTEGRITY error for missing input hash")
}

func TestSOXValidator_ChangeDocumentation_MissingAction(t *testing.T) {
	v := NewSOXValidator()

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "", "sha256:in", "sha256:out", nil),
	}

	violations := v.Validate(entries, nil)

	found := false
	for _, viol := range violations {
		if viol.Rule == "CHANGE_DOCUMENTATION" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected CHANGE_DOCUMENTATION error for missing action")
}

// ---------------------------------------------------------------------------
// ComplianceValidator routing tests
// ---------------------------------------------------------------------------

func TestComplianceValidator_RoutesToCorrectFramework(t *testing.T) {
	cv := NewComplianceValidator()
	ctx := context.Background()
	sopExec := sopdomain.SOPExecution{
		SOPExecutionID: uuid.New(),
		SOPID:          "TEST-01",
		TenantID:       uuid.New(),
		Status:         sopdomain.SOPStatusCompleted,
	}

	// Audit entries with no hashes (should trigger DATA_INTEGRITY from CFR21 and FINANCIAL_DATA_INTEGRITY from SOX).
	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "intake", "", "", nil),
	}

	// Request only HIPAA validation.
	report := cv.Validate(ctx, sopExec, entries, []sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA})
	for _, viol := range report.Violations {
		assert.Equal(t, sopdomain.ComplianceHIPAA, viol.Framework, "all violations should be from HIPAA when only HIPAA is requested")
	}

	// Request only SOX validation.
	report = cv.Validate(ctx, sopExec, entries, []sopdomain.ComplianceFramework{sopdomain.ComplianceSOX})
	for _, viol := range report.Violations {
		assert.Equal(t, sopdomain.ComplianceSOX, viol.Framework, "all violations should be from SOX when only SOX is requested")
	}
}

func TestComplianceValidator_MultipleFrameworks(t *testing.T) {
	cv := NewComplianceValidator()
	ctx := context.Background()
	sopExec := sopdomain.SOPExecution{
		SOPExecutionID: uuid.New(),
		SOPID:          "TEST-01",
		TenantID:       uuid.New(),
		Status:         sopdomain.SOPStatusCompleted,
	}

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "intake", "plaintext_hash", "",
			[]sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA}),
	}

	frameworks := []sopdomain.ComplianceFramework{
		sopdomain.ComplianceHIPAA,
		sopdomain.ComplianceCFR21Part11,
		sopdomain.ComplianceSOX,
	}

	report := cv.Validate(ctx, sopExec, entries, frameworks)

	// Should have violations from multiple frameworks.
	frameworksSeen := make(map[sopdomain.ComplianceFramework]bool)
	for _, viol := range report.Violations {
		frameworksSeen[viol.Framework] = true
	}

	assert.True(t, frameworksSeen[sopdomain.ComplianceHIPAA], "expected HIPAA violations")
	assert.True(t, frameworksSeen[sopdomain.ComplianceCFR21Part11], "expected CFR21 violations")
	assert.True(t, frameworksSeen[sopdomain.ComplianceSOX], "expected SOX violations")
}

func TestComplianceValidator_UnsupportedFramework(t *testing.T) {
	cv := NewComplianceValidator()
	ctx := context.Background()
	sopExec := sopdomain.SOPExecution{
		SOPExecutionID: uuid.New(),
		SOPID:          "TEST-01",
		TenantID:       uuid.New(),
	}

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "intake", "sha256:in", "sha256:out", nil),
	}

	report := cv.Validate(ctx, sopExec, entries, []sopdomain.ComplianceFramework{sopdomain.ComplianceGDPR})

	found := false
	for _, viol := range report.Violations {
		if viol.Rule == "UNSUPPORTED_FRAMEWORK" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected UNSUPPORTED_FRAMEWORK warning for GDPR")
}

func TestComplianceValidator_ValidateWithHITL(t *testing.T) {
	cv := NewComplianceValidator()
	ctx := context.Background()
	sopExec := sopdomain.SOPExecution{
		SOPExecutionID: uuid.New(),
		SOPID:          "TEST-01",
		TenantID:       uuid.New(),
	}

	entries := []sopdomain.AuditEntry{
		newAuditEntry("step-1", sopdomain.StepIntake, "intake", "sha256:in", "sha256:out",
			[]sopdomain.ComplianceFramework{sopdomain.ComplianceCFR21Part11}),
	}

	// HITL request with no decided_by: should trigger CFR21 e-sig violation.
	hitlReqs := []sopdomain.HITLRequest{
		newHITLRequest("decide-step", sopdomain.HITLApproved, nil, timePtr(time.Now())),
	}

	report := cv.ValidateWithHITL(ctx, sopExec, entries, hitlReqs,
		[]sopdomain.ComplianceFramework{sopdomain.ComplianceCFR21Part11})

	found := false
	for _, viol := range report.Violations {
		if viol.Rule == "ELECTRONIC_SIGNATURE" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected ELECTRONIC_SIGNATURE violation from ValidateWithHITL")
	assert.False(t, report.Passed, "report should not pass with ERROR-severity violations")
}

func TestComplianceReport_PassesWhenNoErrors(t *testing.T) {
	cv := NewComplianceValidator()
	ctx := context.Background()
	sopExec := sopdomain.SOPExecution{
		SOPExecutionID: uuid.New(),
		SOPID:          "TEST-01",
		TenantID:       uuid.New(),
	}

	now := time.Now()
	entries := []sopdomain.AuditEntry{
		{
			AuditID:        uuid.New(),
			SOPExecutionID: sopExec.SOPExecutionID,
			SOPID:          "TEST-01",
			TenantID:       sopExec.TenantID,
			StepID:         "intake-step",
			AgentType:      sopdomain.StepIntake,
			Action:         "intake",
			InputHash:      "enc:sha256:abc",
			OutputHash:     "enc:sha256:def",
			ComplianceTags: []sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA},
			CreatedAt:      now,
		},
		{
			AuditID:        uuid.New(),
			SOPExecutionID: sopExec.SOPExecutionID,
			SOPID:          "TEST-01",
			TenantID:       sopExec.TenantID,
			StepID:         "data-step",
			AgentType:      sopdomain.StepDataRetrieval,
			Action:         "fetch_data",
			InputHash:      "enc:sha256:ghi",
			OutputHash:     "enc:sha256:jkl",
			ComplianceTags: []sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA},
			CreatedAt:      now.Add(1 * time.Second),
		},
	}

	report := cv.Validate(ctx, sopExec, entries, []sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA})
	assert.True(t, report.Passed, "report should pass when all HIPAA checks are satisfied")
	assert.False(t, report.CheckedAt.IsZero(), "CheckedAt should be set")
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
