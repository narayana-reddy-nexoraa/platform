package registry

import (
	"time"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// NewHC01PriorAuth returns the SOP for Prior Authorization Submission and Tracking.
func NewHC01PriorAuth() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "HC-01",
		Name:        "Prior Authorization Submission and Tracking",
		Industry:    sopdomain.IndustryHealthcare,
		Version:     "1.0.0",
		Description: "Submit PA requests to payers with clinical documentation, track through approval/denial/peer-to-peer, resolve within clinical urgency SLAs.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "PA Request Intake",
			IntakeDesc:           "Receive order from EHR (CDS hook or manual), identify PA requirement per payer rules, extract procedure/diagnosis codes",
			DataRetrievalName:    "Clinical Document Collection",
			DataRetrievalDesc:    "Gather clinical notes, diagnoses, lab results, imaging reports, letter of medical necessity from EHR chart",
			DataSources:          []string{"Epic", "Cerner", "Meditech", "Payer Portal", "InterQual", "MCG Guidelines"},
			ClassificationName:   "PA Requirement Matching",
			ClassificationDesc:   "Match procedure/diagnosis against payer clinical criteria, identify documentation gaps, assess approval likelihood",
			PromptTemplate:       "prior_auth_matching",
			DecisioningName:      "Submission Readiness Decision",
			DecisioningDesc:      "Determine: submit (all criteria met), request additional clinical docs, or escalate to clinician for LMN",
			HITLAfterDecisioning: true,
			HITLSLADuration:      4 * time.Hour,
			ExecutionName:        "PA Submission",
			ExecutionDesc:        "Submit PA to payer via FHIR API (CRD/DTR/PAS) or portal, capture auth number, link to claim",
			TargetSystems:        []string{"EHR", "PA Management Platform", "Payer Portal", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA},
		ProcessOwner:         "VP Revenue Cycle / Director of Prior Authorization Operations",
		PrimaryUsers:         []string{"PA Coordinator", "Clinical Reviewer", "Physician", "Revenue Cycle Manager"},
		VolumeEstimate:       "500-10,000 PA requests/month",
	}
}

// NewHC02MedicalCoding returns the SOP for Medical Coding and Claim Edit Resolution.
func NewHC02MedicalCoding() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "HC-02",
		Name:        "Medical Coding and Claim Edit Resolution",
		Industry:    sopdomain.IndustryHealthcare,
		Version:     "1.0.0",
		Description: "Review clinical documentation for accurate CPT/ICD-10 coding, resolve claim edits, ensure clean claim submission.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Coding Queue Intake",
			IntakeDesc:           "Receive encounters pending coding review, extract clinical documentation, procedure notes, operative reports",
			DataRetrievalName:    "Clinical Context Pull",
			DataRetrievalDesc:    "Fetch patient history, prior encounters, payer-specific coding rules, NCCI edits, LCD/NCD policies",
			DataSources:          []string{"EHR", "Encoder (3M, Optum)", "CCI Edit Engine", "Payer Rules DB"},
			ClassificationName:   "Code Validation",
			ClassificationDesc:   "Validate CPT/ICD-10 code combinations, check medical necessity, identify unbundling risks, flag audit triggers",
			PromptTemplate:       "medical_coding_validation",
			DecisioningName:      "Edit Resolution",
			DecisioningDesc:      "Resolve claim edits: auto-correct (known patterns), query physician (clinical clarification), or hold for coder review",
			HITLAfterDecisioning: true,
			HITLSLADuration:      8 * time.Hour,
			ExecutionName:        "Clean Claim Submission",
			ExecutionDesc:        "Submit corrected claim to payer, update coding record, log resolution rationale",
			TargetSystems:        []string{"EHR", "Billing System", "Claims Clearinghouse", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA},
		ProcessOwner:         "Director of Health Information Management / Coding Manager",
		PrimaryUsers:         []string{"Medical Coder", "Coding Auditor", "Physician Advisor", "Revenue Integrity Analyst"},
		VolumeEstimate:       "Daily coding volume",
	}
}

// NewHC03Eligibility returns the SOP for Eligibility and Benefits Verification.
func NewHC03Eligibility() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "HC-03",
		Name:        "Eligibility and Benefits Verification",
		Industry:    sopdomain.IndustryHealthcare,
		Version:     "1.0.0",
		Description: "Verify patient insurance eligibility, determine benefit coverage, calculate patient responsibility before service delivery.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Verification Request",
			IntakeDesc:           "Receive verification request (appointment scheduled, walk-in, or pre-service), extract patient demographics and insurance info",
			DataRetrievalName:    "Payer Eligibility Check",
			DataRetrievalDesc:    "Query payer eligibility APIs (270/271 EDI, FHIR), fetch active coverage, benefit details, copay/coinsurance/deductible",
			DataSources:          []string{"Eligibility System", "Payer 270/271 API", "EHR"},
			ClassificationName:   "Coverage Analysis",
			ClassificationDesc:   "Determine coverage status, identify coordination of benefits, calculate estimated patient responsibility",
			PromptTemplate:       "eligibility_analysis",
			DecisioningName:      "Verification Outcome",
			DecisioningDesc:      "Confirm coverage (active), flag issues (inactive/terminated/COB), recommend patient financial counseling if needed",
			HITLAfterDecisioning: false,
			ExecutionName:        "Update Systems",
			ExecutionDesc:        "Update EHR with verified insurance info, set expected patient responsibility, notify scheduling/registration",
			TargetSystems:        []string{"EHR", "Scheduling System", "Patient Portal"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA},
		ProcessOwner:         "Director of Patient Access / Revenue Cycle VP",
		PrimaryUsers:         []string{"Patient Access Representative", "Insurance Verifier", "Financial Counselor"},
		VolumeEstimate:       "10,000+ verifications/month",
	}
}

// NewHC04ReferralMgmt returns the SOP for Referral Management and Care Coordination.
func NewHC04ReferralMgmt() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "HC-04",
		Name:        "Referral Management and Care Coordination",
		Industry:    sopdomain.IndustryHealthcare,
		Version:     "1.0.0",
		Description: "Process specialist referrals, coordinate care transitions, ensure referral completion tracking and loop closure.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Referral Intake",
			IntakeDesc:           "Receive referral order from PCP, extract specialty type, urgency, clinical reason, patient preferences",
			DataRetrievalName:    "Provider and Network Lookup",
			DataRetrievalDesc:    "Find in-network specialists, check availability, verify accepting new patients, match patient preferences",
			DataSources:          []string{"EHR", "Provider Directory", "Insurance Network", "Scheduling System"},
			ClassificationName:   "Referral Priority Assessment",
			ClassificationDesc:   "Classify urgency (routine/urgent/emergent), verify PA requirement, identify care coordination needs",
			PromptTemplate:       "referral_triage",
			DecisioningName:      "Referral Routing",
			DecisioningDesc:      "Route to: auto-schedule (routine, no PA needed), PA workflow (PA required), or care coordinator (complex)",
			HITLAfterDecisioning: false,
			ExecutionName:        "Referral Processing",
			ExecutionDesc:        "Schedule specialist appointment, send clinical summary, notify patient, set follow-up tracker",
			TargetSystems:        []string{"EHR", "Scheduling System", "Patient Portal", "Care Coordination Platform"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceHIPAA},
		ProcessOwner:         "Director of Care Coordination / Medical Director",
		PrimaryUsers:         []string{"Referral Coordinator", "Care Coordinator", "PCP Office Staff"},
		VolumeEstimate:       "Daily referral volume",
	}
}
