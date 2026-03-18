package domain

// ComplianceFramework identifies a regulatory compliance framework.
type ComplianceFramework string

const (
	ComplianceHIPAA       ComplianceFramework = "HIPAA"
	ComplianceCFR21Part11 ComplianceFramework = "21_CFR_PART_11"
	ComplianceBSAAML      ComplianceFramework = "BSA_AML"
	ComplianceSOX         ComplianceFramework = "SOX"
	ComplianceGxP         ComplianceFramework = "GXP"
	ComplianceGVP         ComplianceFramework = "GVP"
	ComplianceISO9001     ComplianceFramework = "ISO_9001"
	ComplianceGDPR        ComplianceFramework = "GDPR"
	ComplianceFDACGMP     ComplianceFramework = "FDA_CGMP"
	ComplianceNAIC        ComplianceFramework = "NAIC"
	ComplianceCMSCoP      ComplianceFramework = "CMS_COP"
)

// RequiresESig returns true if the framework mandates electronic signatures.
func (c ComplianceFramework) RequiresESig() bool {
	return c == ComplianceCFR21Part11
}

// RequiresPHIProtection returns true if PHI must be encrypted and access-logged.
func (c ComplianceFramework) RequiresPHIProtection() bool {
	return c == ComplianceHIPAA || c == ComplianceCMSCoP
}

// RetentionYears returns the minimum record retention period in years.
func (c ComplianceFramework) RetentionYears() int {
	switch c {
	case ComplianceBSAAML:
		return 7
	case ComplianceHIPAA:
		return 6
	case ComplianceSOX:
		return 7
	case ComplianceCFR21Part11:
		return 10
	case ComplianceGDPR:
		return 5
	default:
		return 5
	}
}
