package registry

import (
	"time"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// NewLS01Pharmacovigilance returns the SOP for Pharmacovigilance Case Intake and Triage.
func NewLS01Pharmacovigilance() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "LS-01",
		Name:        "Pharmacovigilance Case Intake and Triage",
		Industry:    sopdomain.IndustryLifeSciences,
		Version:     "1.0.0",
		Description: "Receive ICSRs via any channel, deduplicate, triage seriousness/expectedness, code with MedDRA, enter into safety database, route to medical reviewer within regulatory deadlines (7-day/15-day).",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "ICSR Intake",
			IntakeDesc:           "Receive Individual Case Safety Report via E2B gateway, email, fax, literature, partner portal, or patient support program",
			DataRetrievalName:    "Product and Case Data Pull",
			DataRetrievalDesc:    "Fetch product reference data, CCDS/RSI, MedDRA dictionary, duplicate case index, partner agreements",
			DataSources:          []string{"Safety Database (Argus/Veeva)", "MedDRA MSSO", "CCDS Registry", "Literature Monitoring Platform"},
			ClassificationName:   "Seriousness and Expectedness Triage",
			ClassificationDesc:   "Assess seriousness (serious/non-serious), expectedness (listed/unlisted), determine reporting timeline (7/15/90 day), MedDRA code adverse events",
			PromptTemplate:       "pv_case_triage",
			DecisioningName:      "Medical Reviewer Assignment",
			DecisioningDesc:      "Route to medical reviewer queue by priority tier; medical reviewer confirms seriousness, causality, and authorizes E2B submission",
			HITLAfterDecisioning: true,
			HITLSLADuration:      24 * time.Hour,
			ExecutionName:        "E2B Submission",
			ExecutionDesc:        "Generate E2B R3 XML, obtain medical reviewer e-signature (21 CFR Part 11), transmit to regulatory authorities (EudraVigilance, FDA ESG)",
			TargetSystems:        []string{"Safety Database", "EudraVigilance EVWEB", "FDA ESG", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceCFR21Part11, sopdomain.ComplianceGVP, sopdomain.ComplianceGxP},
		ProcessOwner:         "Head of Global Pharmacovigilance / Drug Safety Director",
		PrimaryUsers:         []string{"PV Data Entry Specialist", "Medical Reviewer (MD/PharmD)", "Signal Detection Scientist", "QPPV"},
		VolumeEstimate:       "100-50,000+ ICSRs/month",
	}
}

// NewLS02ProductComplaints returns the SOP for Product Complaint Handling and Triage.
func NewLS02ProductComplaints() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "LS-02",
		Name:        "Product Complaint Handling and Triage",
		Industry:    sopdomain.IndustryLifeSciences,
		Version:     "1.0.0",
		Description: "Receive product quality complaints, triage for severity and reportability, initiate investigation, and report to regulatory authorities as required.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Complaint Intake",
			IntakeDesc:           "Receive product complaint via call center, email, web form, or field report; extract product, lot, defect description",
			DataRetrievalName:    "Product and History Lookup",
			DataRetrievalDesc:    "Fetch product master, lot/batch records, prior complaints for same product/lot, trending data",
			DataSources:          []string{"QMS (Veeva/TrackWise)", "ERP", "Product Master", "Complaint History DB"},
			ClassificationName:   "Severity and Reportability Assessment",
			ClassificationDesc:   "Classify complaint severity, determine MDR/MedWatch reportability, identify trending signals",
			PromptTemplate:       "product_complaint_triage",
			DecisioningName:      "Investigation Decision",
			DecisioningDesc:      "Determine: close (non-actionable), investigate (quality event), report (MDR/MedWatch), or recall assessment",
			HITLAfterDecisioning: true,
			HITLSLADuration:      24 * time.Hour,
			ExecutionName:        "Complaint Processing",
			ExecutionDesc:        "Create investigation record, initiate CAPA if warranted, submit regulatory report, notify quality team",
			TargetSystems:        []string{"QMS", "Regulatory Portal", "ERP", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceCFR21Part11, sopdomain.ComplianceGxP},
		ProcessOwner:         "VP Quality / Director of Quality Assurance",
		PrimaryUsers:         []string{"Complaint Coordinator", "Quality Investigator", "Regulatory Affairs Specialist"},
		VolumeEstimate:       "Varies by product portfolio",
	}
}

// NewLS03RegulatorySubmission returns the SOP for Regulatory Submission Content Assembly.
func NewLS03RegulatorySubmission() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "LS-03",
		Name:        "Regulatory Submission Content Assembly Support",
		Industry:    sopdomain.IndustryLifeSciences,
		Version:     "1.0.0",
		Description: "Assemble regulatory submission packages (eCTD modules), verify document completeness, manage publishing lifecycle through submission.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Submission Request",
			IntakeDesc:           "Receive submission request with target authority, submission type (NDA/BLA/MAA/variation), and document list",
			DataRetrievalName:    "Document Collection",
			DataRetrievalDesc:    "Gather required documents from document management, clinical data, manufacturing records, nonclinical reports",
			DataSources:          []string{"Veeva Vault", "EDMS", "Clinical Data Warehouse", "Manufacturing Records"},
			ClassificationName:   "Completeness and Compliance Check",
			ClassificationDesc:   "Validate eCTD structure, check document versions, verify cross-references, identify missing sections",
			PromptTemplate:       "regulatory_submission_check",
			DecisioningName:      "Submission Readiness Review",
			DecisioningDesc:      "Regulatory affairs reviewer confirms completeness, approves for publishing and submission",
			HITLAfterDecisioning: true,
			HITLSLADuration:      48 * time.Hour,
			ExecutionName:        "Submission Publishing",
			ExecutionDesc:        "Publish eCTD sequence, validate XML backbone, submit to regulatory authority gateway, confirm receipt",
			TargetSystems:        []string{"eCTD Publishing Tool", "Regulatory Authority Gateway", "Document Management", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceCFR21Part11},
		ProcessOwner:         "VP Regulatory Affairs / Head of Regulatory Operations",
		PrimaryUsers:         []string{"Regulatory Affairs Manager", "Submissions Coordinator", "Publishing Specialist", "Medical Writer"},
		VolumeEstimate:       "Per-submission cadence",
	}
}

// NewLS04QualityCapa returns the SOP for Quality Event Triage (Deviation and CAPA).
func NewLS04QualityCapa() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "LS-04",
		Name:        "Quality Event Triage (Deviation and CAPA Initiation)",
		Industry:    sopdomain.IndustryLifeSciences,
		Version:     "1.0.0",
		Description: "Receive quality events (deviations, OOS results, audit observations), triage severity, initiate CAPA when warranted, track through closure.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Quality Event Report",
			IntakeDesc:           "Receive deviation/OOS/observation report, extract event description, affected product/lot, immediate actions taken",
			DataRetrievalName:    "Quality History Pull",
			DataRetrievalDesc:    "Fetch related deviations, trending data, product impact analysis, prior CAPAs for same root cause category",
			DataSources:          []string{"QMS (Veeva/TrackWise/MasterControl)", "ERP", "Batch Records", "CAPA History"},
			ClassificationName:   "Severity Classification",
			ClassificationDesc:   "Classify event severity (critical/major/minor), assess product impact, determine if CAPA warranted, check regulatory reporting trigger",
			PromptTemplate:       "quality_event_triage",
			DecisioningName:      "CAPA Decision",
			DecisioningDesc:      "Determine: close with justification (minor), initiate CAPA (major/critical), or escalate to management review",
			HITLAfterDecisioning: true,
			HITLSLADuration:      24 * time.Hour,
			ExecutionName:        "CAPA Initiation",
			ExecutionDesc:        "Create CAPA record, assign owner, set root cause investigation timeline, notify quality council, place product hold if needed",
			TargetSystems:        []string{"QMS", "ERP (lot hold)", "Quality Council Notification", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceCFR21Part11, sopdomain.ComplianceGxP, sopdomain.ComplianceFDACGMP},
		ProcessOwner:         "VP Quality / Director of Quality Systems",
		PrimaryUsers:         []string{"Quality Event Coordinator", "CAPA Owner", "Quality Investigator", "QA Manager"},
		VolumeEstimate:       "Varies by manufacturing volume",
	}
}
