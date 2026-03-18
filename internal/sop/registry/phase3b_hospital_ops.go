package registry

import (
	"time"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// NewHOSP01BedMgmt returns the SOP for Inpatient Bed Management and Patient Flow.
func NewHOSP01BedMgmt() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "HOSP-01",
		Name:        "Inpatient Bed Management and Patient Flow Optimization",
		Industry:    sopdomain.IndustryHospitalOps,
		Version:     "1.0.0",
		Description: "Assign inpatient beds within clinical urgency SLA, minimize ED boarding, predict capacity 4-8 hours ahead, manage 24/7 patient flow.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "ADT Event Intake",
			IntakeDesc:           "Receive ADT events (admission order, discharge, transfer request, ED boarding alert), extract patient acuity and bed requirements",
			DataRetrievalName:    "Capacity Data Pull",
			DataRetrievalDesc:    "Fetch real-time bed status, unit census, staffed beds, isolation rooms, ED boarding list, predicted discharges",
			DataSources:          []string{"EHR ADT Feed", "Bed Management System", "Patient Flow Platform (Qventus/TeleTracking)", "ED Tracking Board"},
			ClassificationName:   "Bed Assignment Recommendation",
			ClassificationDesc:   "AI-assisted bed matching: diagnosis→unit, isolation requirements, proximity to care team, capacity forecast",
			PromptTemplate:       "bed_assignment",
			DecisioningName:      "Bed Assignment Decision",
			DecisioningDesc:      "House supervisor reviews AI recommendation, confirms or overrides bed assignment",
			HITLAfterDecisioning: true,
			HITLSLADuration:      15 * time.Minute,
			ExecutionName:        "Bed Assignment Execution",
			ExecutionDesc:        "Execute ADT transfer in EHR, trigger EVS dispatch for room cleaning, notify receiving unit charge nurse",
			TargetSystems:        []string{"EHR", "Bed Management", "EVS System", "Paging System (Vocera/Spok)"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceCMSCoP, sopdomain.ComplianceHIPAA},
		ProcessOwner:         "Chief Nursing Officer / VP Patient Care Services",
		PrimaryUsers:         []string{"House Supervisor", "Bed Placement Coordinator", "Charge Nurse", "ED Charge Nurse", "Transfer Center Staff"},
		VolumeEstimate:       "Continuous 24/7, decisions every 5-30 minutes",
	}
}

// NewHOSP02Discharge returns the SOP for Discharge Planning Coordination.
func NewHOSP02Discharge() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "HOSP-02",
		Name:        "Discharge Planning Coordination",
		Industry:    sopdomain.IndustryHospitalOps,
		Version:     "1.0.0",
		Description: "Predict discharge readiness, coordinate multidisciplinary discharge plans, ensure safe transitions to reduce readmissions.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Discharge Trigger",
			IntakeDesc:           "Receive discharge prediction or physician discharge order, extract patient clinical status and discharge barriers",
			DataRetrievalName:    "Discharge Readiness Data",
			DataRetrievalDesc:    "Fetch pending tests, medication reconciliation status, patient education, home care arrangements, transport",
			DataSources:          []string{"EHR", "Patient Flow Platform", "Social Work System", "Pharmacy System"},
			ClassificationName:   "Discharge Barrier Analysis",
			ClassificationDesc:   "Identify blocking factors (pending labs, family meeting needed, DME delivery, SNF bed availability)",
			PromptTemplate:       "discharge_planning",
			DecisioningName:      "Discharge Plan Approval",
			DecisioningDesc:      "Charge nurse reviews discharge readiness, confirms plan completeness, clears barriers",
			HITLAfterDecisioning: true,
			HITLSLADuration:      2 * time.Hour,
			ExecutionName:        "Discharge Execution",
			ExecutionDesc:        "Process discharge in EHR (ADT A03), generate discharge instructions, arrange transport, notify PCP, trigger bed cleanup",
			TargetSystems:        []string{"EHR", "Patient Portal", "PCP Notification", "EVS System"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceCMSCoP, sopdomain.ComplianceHIPAA},
		ProcessOwner:         "Director of Case Management / CNO",
		PrimaryUsers:         []string{"Case Manager", "Discharge Planner", "Charge Nurse", "Social Worker"},
		VolumeEstimate:       "Daily per unit",
	}
}

// NewHOSP03ORScheduling returns the SOP for OR Scheduling Optimization.
func NewHOSP03ORScheduling() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "HOSP-03",
		Name:        "OR Scheduling Optimization and Block Time Management",
		Industry:    sopdomain.IndustryHospitalOps,
		Version:     "1.0.0",
		Description: "Optimize OR scheduling, manage surgeon block time utilization, predict case durations, maximize OR throughput.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Case Booking Request",
			IntakeDesc:           "Receive surgical case booking, extract procedure code (CPT), surgeon, estimated duration, equipment needs, patient status",
			DataRetrievalName:    "Scheduling Context",
			DataRetrievalDesc:    "Fetch OR availability, surgeon block allocations, equipment inventory, anesthesia availability, historical case durations",
			DataSources:          []string{"OR Scheduling System", "EHR (Epic Cadence)", "Equipment Tracking", "Surgeon Block Calendar"},
			ClassificationName:   "Case Duration Prediction and Conflict Detection",
			ClassificationDesc:   "AI-predict case duration, detect scheduling conflicts, assess block time utilization rates",
			PromptTemplate:       "or_scheduling",
			DecisioningName:      "Schedule Optimization",
			DecisioningDesc:      "Recommend optimal slot, flag block underutilization for release, suggest case reordering for turnover reduction",
			HITLAfterDecisioning: true,
			HITLSLADuration:      4 * time.Hour,
			ExecutionName:        "Schedule Finalization",
			ExecutionDesc:        "Book OR slot, notify surgeon office and anesthesia, update daily OR schedule board, handle cancellation and slot recovery",
			TargetSystems:        []string{"OR Scheduling", "EHR", "Surgeon Portal", "Anesthesia Scheduling"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceCMSCoP},
		ProcessOwner:         "Director of Perioperative Services / VP Surgical Services",
		PrimaryUsers:         []string{"OR Scheduler", "Surgeon Coordinator", "Perioperative Director", "Block Time Committee"},
		VolumeEstimate:       "Daily scheduling",
	}
}

// NewHOSP04SupplyChain returns the SOP for Hospital Supply Chain Replenishment.
func NewHOSP04SupplyChain() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "HOSP-04",
		Name:        "Hospital Supply Chain Replenishment and Inventory Management",
		Industry:    sopdomain.IndustryHospitalOps,
		Version:     "1.0.0",
		Description: "Monitor supply levels, automate replenishment orders, manage critical supply alerts, optimize par levels across units.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Inventory Alert",
			IntakeDesc:           "Receive low-stock alert or scheduled par level review trigger, extract item details and unit location",
			DataRetrievalName:    "Supply Data Pull",
			DataRetrievalDesc:    "Fetch current inventory levels, consumption rates, open POs, vendor lead times, contract pricing",
			DataSources:          []string{"Materials Management System", "ERP (Oracle SCM/Ariba)", "Vendor Portals"},
			ClassificationName:   "Criticality Assessment",
			ClassificationDesc:   "Classify supply urgency: routine reorder, critical shortage (patient safety), substitute available, backorder",
			PromptTemplate:       "supply_chain_triage",
			DecisioningName:      "Replenishment Decision",
			DecisioningDesc:      "Auto-reorder (routine), escalate (critical shortage with no substitute), or recommend substitution",
			HITLAfterDecisioning: true,
			HITLSLADuration:      1 * time.Hour,
			ExecutionName:        "Order Placement",
			ExecutionDesc:        "Place PO with vendor, update par levels, notify unit manager, trigger substitution workflow if needed",
			TargetSystems:        []string{"Materials Management", "ERP", "Vendor Portal", "Unit Manager Notification"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceCMSCoP},
		ProcessOwner:         "Director of Materials Management / VP Supply Chain",
		PrimaryUsers:         []string{"Supply Chain Analyst", "Materials Manager", "Unit Supply Coordinator", "Procurement Specialist"},
		VolumeEstimate:       "Daily + alert-triggered",
	}
}
