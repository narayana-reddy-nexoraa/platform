package registry

import (
	"time"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// NewFS01KYC returns the SOP definition for KYC Customer Onboarding and Due Diligence.
func NewFS01KYC() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "FS-01",
		Name:        "KYC Customer Onboarding and Due Diligence",
		Industry:    sopdomain.IndustryFinancialServices,
		Version:     "1.0.0",
		Description: "Verify customer identity, assess risk tier, collect documentation, and activate or review customer record in CLM system within SLA with regulator-ready audit trail.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Application Intake",
			IntakeDesc:           "Receive customer application (web form, PDF, API), extract identity fields, validate completeness",
			DataRetrievalName:    "Screening and Verification",
			DataRetrievalDesc:    "Run PEP/sanctions screening (OFAC, EU, UN), adverse media check, UBO resolution, identity verification",
			DataSources:          []string{"LexisNexis", "Dow Jones RiskCenter", "Refinitiv World-Check", "Jumio", "Socure", "Dun & Bradstreet"},
			ClassificationName:   "Risk Tier Classification",
			ClassificationDesc:   "Classify customer as Low/Medium/High/PEP based on screening results, jurisdiction, entity type",
			PromptTemplate:       "kyc_risk_classification",
			DecisioningName:      "Due Diligence Decision",
			DecisioningDesc:      "Determine: auto-approve (Low), require human review (Medium/High/PEP), or reject",
			HITLAfterDecisioning: true,
			HITLSLADuration:      4 * time.Hour,
			ExecutionName:        "Customer Activation",
			ExecutionDesc:        "Activate customer in CLM, create compliance record, notify relationship manager",
			TargetSystems:        []string{"Fenergo", "Salesforce", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceBSAAML, sopdomain.ComplianceSOX, sopdomain.ComplianceGDPR},
		ProcessOwner:         "Chief Compliance Officer / Head of KYC Operations",
		PrimaryUsers:         []string{"KYC Analyst", "Compliance Reviewer", "Relationship Manager", "Risk Officer"},
		VolumeEstimate:       "500-5,000 new cases/month",
	}
}

// NewFS02AML returns the SOP definition for AML Transaction Alert Triage.
func NewFS02AML() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "FS-02",
		Name:        "AML Transaction Alert Triage",
		Industry:    sopdomain.IndustryFinancialServices,
		Version:     "1.0.0",
		Description: "Triage AML transaction monitoring alerts, investigate suspicious activity, determine SAR filing requirement with complete evidence trail.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Alert Intake",
			IntakeDesc:           "Receive transaction monitoring alert, extract alert details, customer context, and triggering rules",
			DataRetrievalName:    "Transaction and Customer Data Pull",
			DataRetrievalDesc:    "Fetch transaction history, customer profile, prior alerts, screening results, account activity patterns",
			DataSources:          []string{"Core Banking", "Transaction Monitoring System", "LexisNexis", "OFAC Screening"},
			ClassificationName:   "Alert Triage and Risk Scoring",
			ClassificationDesc:   "Score alert severity, identify fraud indicators, classify transaction patterns, detect structuring",
			PromptTemplate:       "aml_alert_triage",
			DecisioningName:      "SAR Filing Decision",
			DecisioningDesc:      "Determine: close as false positive, escalate to Tier 2, or recommend SAR filing",
			HITLAfterDecisioning: true,
			HITLSLADuration:      24 * time.Hour,
			ExecutionName:        "Alert Disposition",
			ExecutionDesc:        "Close alert with rationale, escalate to investigation, or initiate SAR filing workflow",
			TargetSystems:        []string{"Case Management", "SAR Filing System", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceBSAAML},
		ProcessOwner:         "BSA Officer / Head of Financial Crime Investigations",
		PrimaryUsers:         []string{"Tier 1 AML Analyst", "Tier 2 Investigator", "BSA Officer"},
		VolumeEstimate:       "Thousands of alerts/day",
	}
}

// NewFS03TradeRecon returns the SOP definition for Trade and Position Reconciliation.
func NewFS03TradeRecon() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "FS-03",
		Name:        "Trade and Position Reconciliation",
		Industry:    sopdomain.IndustryFinancialServices,
		Version:     "1.0.0",
		Description: "Reconcile trade positions across front/middle/back office systems, identify and resolve breaks within SLA.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Reconciliation Trigger",
			IntakeDesc:           "Receive end-of-day or intraday recon trigger, identify scope (asset class, entity, date range)",
			DataRetrievalName:    "Position Data Extraction",
			DataRetrievalDesc:    "Extract positions from trading system, prime broker, custodian, and accounting system",
			DataSources:          []string{"Trading System", "Prime Broker", "Custodian", "Accounting System"},
			ClassificationName:   "Break Detection and Classification",
			ClassificationDesc:   "Match positions, identify breaks, classify by type (quantity, price, missing trade, timing)",
			PromptTemplate:       "trade_recon_classification",
			DecisioningName:      "Break Resolution Routing",
			DecisioningDesc:      "Route breaks: auto-resolve (known patterns), escalate (material breaks), investigate (unknown)",
			HITLAfterDecisioning: true,
			HITLSLADuration:      4 * time.Hour,
			ExecutionName:        "Break Resolution",
			ExecutionDesc:        "Apply corrections, book adjustments, notify counterparties",
			TargetSystems:        []string{"Accounting System", "Trading System", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceSOX},
		ProcessOwner:         "Head of Operations / Chief Operating Officer",
		PrimaryUsers:         []string{"Reconciliation Analyst", "Operations Manager", "Middle Office"},
		VolumeEstimate:       "Daily reconciliation cycles",
	}
}

// NewFS04RegulatoryReporting returns the SOP definition for Regulatory Reporting.
func NewFS04RegulatoryReporting() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "FS-04",
		Name:        "Regulatory Reporting Workflow (DFAST/CCAR/SOX)",
		Industry:    sopdomain.IndustryFinancialServices,
		Version:     "1.0.0",
		Description: "Prepare, validate, and submit regulatory reports (DFAST stress tests, CCAR capital plans, SOX controls) with full audit trail.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Reporting Trigger",
			IntakeDesc:           "Receive regulatory reporting trigger (scheduled or ad-hoc), identify report type and parameters",
			DataRetrievalName:    "Data Collection and Assembly",
			DataRetrievalDesc:    "Gather financial data, model outputs, scenario results from data warehouse and risk systems",
			DataSources:          []string{"Data Warehouse", "Risk Engine", "Capital Planning System", "General Ledger"},
			ClassificationName:   "Data Validation and Quality Check",
			ClassificationDesc:   "Validate data completeness, run quality rules, identify gaps and anomalies",
			PromptTemplate:       "regulatory_report_validation",
			DecisioningName:      "Report Approval",
			DecisioningDesc:      "Review report for accuracy, completeness; determine submission readiness",
			HITLAfterDecisioning: true,
			HITLSLADuration:      48 * time.Hour,
			ExecutionName:        "Report Submission",
			ExecutionDesc:        "Format report per regulatory template, submit to regulator portal, confirm receipt",
			TargetSystems:        []string{"Regulatory Portal", "Document Management", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceSOX},
		ProcessOwner:         "Chief Risk Officer / Head of Regulatory Reporting",
		PrimaryUsers:         []string{"Regulatory Reporting Analyst", "Risk Manager", "Finance Controller"},
		VolumeEstimate:       "Quarterly + ad-hoc submissions",
	}
}
