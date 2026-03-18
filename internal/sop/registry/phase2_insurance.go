package registry

import (
	"time"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// NewINS01FNOL returns the SOP definition for First Notice of Loss Intake and Triage.
func NewINS01FNOL() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "INS-01",
		Name:        "First Notice of Loss (FNOL) Intake and Triage",
		Industry:    sopdomain.IndustryInsurance,
		Version:     "1.0.0",
		Description: "Receive FNOL via any channel (voice, email, chat, web), verify policy, triage severity, detect fraud indicators, create claim, and assign adjuster.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "FNOL Intake",
			IntakeDesc:           "Receive loss report via voice/email/chat/web, extract policy number, date of loss, loss type, claimant info, loss description",
			DataRetrievalName:    "Policy Verification",
			DataRetrievalDesc:    "Fetch policy details, coverage limits, deductibles, endorsements, payment history, prior claims",
			DataSources:          []string{"Policy Admin System", "Billing System", "Claims History"},
			ClassificationName:   "Severity Triage and Fraud Detection",
			ClassificationDesc:   "Classify loss severity (Low/Medium/High), detect fraud indicators, flag CAT events, assign priority tier",
			PromptTemplate:       "fnol_triage",
			DecisioningName:      "Claim Routing Decision",
			DecisioningDesc:      "Determine: auto-create claim (low severity), escalate to senior adjuster (high/fraud), request additional info",
			HITLAfterDecisioning: true,
			HITLSLADuration:      2 * time.Hour,
			ExecutionName:        "Claim Creation and Assignment",
			ExecutionDesc:        "Create claim record, set initial reserves, assign adjuster, notify claimant and supervising manager",
			TargetSystems:        []string{"Claims Management System", "Notification Service", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceNAIC},
		ProcessOwner:         "VP Claims Operations",
		PrimaryUsers:         []string{"FNOL Representative", "Claims Supervisor", "Senior Adjuster", "SIU Analyst"},
		VolumeEstimate:       "5,000-50,000 FNOLs/month",
	}
}

// NewINS02Underwriting returns the SOP definition for Underwriting Submission Triage.
func NewINS02Underwriting() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "INS-02",
		Name:        "Underwriting Submission Triage and Intake",
		Industry:    sopdomain.IndustryInsurance,
		Version:     "1.0.0",
		Description: "Receive underwriting submissions, validate completeness, assess risk appetite fit, route to appropriate underwriter with risk analysis.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Submission Intake",
			IntakeDesc:           "Receive submission from broker/agent, extract risk details, validate document completeness",
			DataRetrievalName:    "Risk Data Enrichment",
			DataRetrievalDesc:    "Pull loss history, industry benchmarks, catastrophe exposure, prior policy data, credit scores",
			DataSources:          []string{"Policy Admin", "Loss History DB", "Third-Party Risk Data", "CAT Modeling"},
			ClassificationName:   "Risk Appetite Assessment",
			ClassificationDesc:   "Score submission against risk appetite guidelines, classify by line of business, exposure tier",
			PromptTemplate:       "underwriting_triage",
			DecisioningName:      "Routing Decision",
			DecisioningDesc:      "Route to: auto-decline (outside appetite), auto-bind (within guidelines), or underwriter review",
			HITLAfterDecisioning: true,
			HITLSLADuration:      24 * time.Hour,
			ExecutionName:        "Submission Processing",
			ExecutionDesc:        "Route to underwriter queue, create submission record, generate preliminary analysis, notify broker",
			TargetSystems:        []string{"Underwriting Workbench", "Broker Portal", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceNAIC},
		ProcessOwner:         "Chief Underwriting Officer",
		PrimaryUsers:         []string{"Submission Analyst", "Underwriter", "Underwriting Manager"},
		VolumeEstimate:       "Varies by line of business",
	}
}

// NewINS03ClaimsAdjudication returns the SOP definition for Claims Adjudication Support.
func NewINS03ClaimsAdjudication() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "INS-03",
		Name:        "Claims Adjudication Support and Auto-Adjudication",
		Industry:    sopdomain.IndustryInsurance,
		Version:     "1.0.0",
		Description: "Evaluate claims against policy terms, apply coverage rules, calculate payment, and auto-adjudicate within threshold or route to human adjuster.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Claim Review Trigger",
			IntakeDesc:           "Receive claim ready for adjudication, extract claim details, loss documentation, repair estimates",
			DataRetrievalName:    "Coverage and Documentation Pull",
			DataRetrievalDesc:    "Fetch policy terms, coverage limits, deductibles, endorsements, claim documentation, vendor estimates",
			DataSources:          []string{"Claims System", "Policy Admin", "Document Management", "Vendor Network"},
			ClassificationName:   "Coverage Analysis",
			ClassificationDesc:   "Analyze coverage applicability, identify exclusions, calculate covered amount, flag subrogation potential",
			PromptTemplate:       "claims_adjudication",
			DecisioningName:      "Payment Decision",
			DecisioningDesc:      "Determine payment amount; auto-adjudicate if within threshold, else route to human adjuster",
			HITLAfterDecisioning: true,
			HITLSLADuration:      8 * time.Hour,
			ExecutionName:        "Payment Processing",
			ExecutionDesc:        "Issue payment, update reserves, close or hold claim, notify claimant",
			TargetSystems:        []string{"Claims System", "Payment System", "Notification Service"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceNAIC},
		ProcessOwner:         "VP Claims Operations",
		PrimaryUsers:         []string{"Claims Adjuster", "Claims Examiner", "Claims Manager"},
		VolumeEstimate:       "Varies by claim volume",
	}
}

// NewINS04Subrogation returns the SOP definition for Subrogation Identification and Recovery.
func NewINS04Subrogation() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "INS-04",
		Name:        "Subrogation Identification and Recovery Workflow",
		Industry:    sopdomain.IndustryInsurance,
		Version:     "1.0.0",
		Description: "Identify subrogation opportunities in paid claims, initiate recovery actions against third parties, track recovery through completion.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Subrogation Screening",
			IntakeDesc:           "Screen closed/paid claims for subrogation potential, extract liable third-party indicators",
			DataRetrievalName:    "Liability Evidence Collection",
			DataRetrievalDesc:    "Gather police reports, witness statements, liability assessments, third-party insurance info",
			DataSources:          []string{"Claims System", "Document Management", "Third-Party Insurance DB"},
			ClassificationName:   "Recovery Potential Assessment",
			ClassificationDesc:   "Score recovery likelihood, estimate recovery amount, classify by complexity (demand letter vs. litigation)",
			PromptTemplate:       "subrogation_assessment",
			DecisioningName:      "Recovery Strategy Decision",
			DecisioningDesc:      "Determine: pursue demand letter, refer to litigation, or close (insufficient evidence)",
			HITLAfterDecisioning: true,
			HITLSLADuration:      48 * time.Hour,
			ExecutionName:        "Recovery Initiation",
			ExecutionDesc:        "Send demand letter, create recovery case, assign to subrogation specialist, set follow-up schedule",
			TargetSystems:        []string{"Subrogation System", "Document Management", "Notification Service"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceNAIC},
		ProcessOwner:         "VP Subrogation / Recovery Director",
		PrimaryUsers:         []string{"Subrogation Specialist", "Subrogation Manager", "Legal Counsel"},
		VolumeEstimate:       "Varies by claim volume",
	}
}

// NewCounterpartyRisk returns the standalone Counterparty Risk Assessment SOP.
func NewCounterpartyRisk() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "CPR-01",
		Name:        "Counterparty Risk Assessment",
		Industry:    sopdomain.IndustryFinancialServices,
		Version:     "1.0.0",
		Description: "Assess counterparty (beneficiary/receiver) risk for transaction monitoring and suspicious activity investigations.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Counterparty Identification",
			IntakeDesc:           "Identify beneficiary name, country, receiving bank, new/existing status, declared relationship",
			DataRetrievalName:    "Risk Factor Data Pull",
			DataRetrievalDesc:    "Fetch jurisdiction risk rating, sanctions exposure, entity ownership structure, transaction history",
			DataSources:          []string{"OFAC", "Country Risk DB", "Entity Registry", "Transaction History"},
			ClassificationName:   "Counterparty Classification",
			ClassificationDesc:   "Classify as: Internal Customer, Known External, New External, Intermediary, or High-Risk Entity",
			PromptTemplate:       "counterparty_risk_classification",
			DecisioningName:      "Risk Rating Assignment",
			DecisioningDesc:      "Assign risk rating (Low/Medium/High/Prohibited) based on jurisdiction, entity, relationship, behavior",
			HITLAfterDecisioning: true,
			HITLSLADuration:      4 * time.Hour,
			ExecutionName:        "Disposition Action",
			ExecutionDesc:        "Apply action: proceed (Low), enhanced monitoring (Medium), escalate (High), restrict (Prohibited)",
			TargetSystems:        []string{"Case Management", "Transaction Monitoring", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceBSAAML},
		ProcessOwner:         "BSA Officer / Head of Transaction Monitoring",
		PrimaryUsers:         []string{"Tier 1 Analyst", "Tier 2 Investigator", "Compliance Officer"},
		VolumeEstimate:       "Per-transaction, continuous",
	}
}
