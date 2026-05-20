package registry

import (
	"time"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// NewMFG01WorkOrders returns the SOP for Production Work Order Management and Scheduling.
func NewMFG01WorkOrders() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "MFG-01",
		Name:        "Production Work Order Management and Scheduling",
		Industry:    sopdomain.IndustryManufacturing,
		Version:     "1.0.0",
		Description: "Translate demand into capacity-feasible work orders, optimize production schedule, manage material availability, and track through completion.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "MRP Output Processing",
			IntakeDesc:           "Receive planned orders from ERP MRP/MPS run, extract item, quantity, required date, BOM/routing version",
			DataRetrievalName:    "Capacity and Material Check",
			DataRetrievalDesc:    "Fetch work center capacity, BOM components, inventory levels, labor availability, open PO status",
			DataSources:          []string{"SAP S/4HANA", "Oracle Cloud Manufacturing", "APS (Kinaxis/Preactor)", "Inventory System"},
			ClassificationName:   "Schedule Feasibility Analysis",
			ClassificationDesc:   "AI-assisted finite capacity scheduling, detect conflicts (overload, material shortage, tooling), sequence optimization",
			PromptTemplate:       "work_order_scheduling",
			DecisioningName:      "Schedule Confirmation",
			DecisioningDesc:      "Scheduler reviews AI-generated schedule, confirms or adjusts work order dates and sequences",
			HITLAfterDecisioning: true,
			HITLSLADuration:      4 * time.Hour,
			ExecutionName:        "Work Order Release",
			ExecutionDesc:        "Release firm work orders to MES, reserve components, assign labor/machine, notify production supervisor",
			TargetSystems:        []string{"ERP", "MES (Opcenter/FactoryTalk)", "APS", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceISO9001},
		ProcessOwner:         "VP Manufacturing / Director of Production Planning",
		PrimaryUsers:         []string{"Production Planner", "Scheduler", "Production Supervisor", "Manufacturing Engineer"},
		VolumeEstimate:       "500-50,000 active work orders simultaneously",
	}
}

// NewMFG02SPCQuality returns the SOP for Statistical Process Control and Quality Alert Management.
func NewMFG02SPCQuality() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "MFG-02",
		Name:        "Statistical Process Control and Quality Alert Management",
		Industry:    sopdomain.IndustryManufacturing,
		Version:     "1.0.0",
		Description: "Monitor real-time SPC data from production, detect out-of-control conditions, generate quality alerts, and initiate containment actions.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "SPC Data Intake",
			IntakeDesc:           "Receive measurement data from sensors/CMM/manual gauges via OPC-UA or MQTT, extract control chart parameters",
			DataRetrievalName:    "Control Limits and History Pull",
			DataRetrievalDesc:    "Fetch control limits, specification limits, process capability history, recent measurement trends",
			DataSources:          []string{"SPC System (Minitab/InfinityQS)", "MES", "IoT Platform", "Quality DB"},
			ClassificationName:   "Out-of-Control Detection",
			ClassificationDesc:   "Apply Western Electric rules, detect trends, shifts, runs; classify alert severity (warning/violation/critical)",
			PromptTemplate:       "spc_alert_classification",
			DecisioningName:      "Containment Decision",
			DecisioningDesc:      "Determine: adjust process (minor drift), halt production and investigate (violation), or quarantine lot (critical)",
			HITLAfterDecisioning: true,
			HITLSLADuration:      30 * time.Minute,
			ExecutionName:        "Quality Action Execution",
			ExecutionDesc:        "Issue quality hold, quarantine affected lots, create investigation record, notify quality and production teams",
			TargetSystems:        []string{"QMS", "MES", "ERP (lot hold)", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceISO9001, sopdomain.ComplianceFDACGMP},
		ProcessOwner:         "VP Quality / Director of Manufacturing Quality",
		PrimaryUsers:         []string{"Quality Engineer", "SPC Analyst", "Production Supervisor", "Quality Manager"},
		VolumeEstimate:       "Real-time sensor data, continuous",
	}
}

// NewMFG03PredictiveMaint returns the SOP for Predictive Maintenance Work Order Generation.
func NewMFG03PredictiveMaint() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "MFG-03",
		Name:        "Predictive Maintenance Work Order Generation",
		Industry:    sopdomain.IndustryManufacturing,
		Version:     "1.0.0",
		Description: "Analyze equipment sensor data, predict failures, generate proactive maintenance work orders to prevent unplanned downtime.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Sensor Alert Intake",
			IntakeDesc:           "Receive equipment health alert from IoT platform (vibration, temperature, pressure anomalies), extract asset and sensor details",
			DataRetrievalName:    "Equipment History Pull",
			DataRetrievalDesc:    "Fetch maintenance history, last PM date, spare parts inventory, production schedule impact, MTBF/MTTR data",
			DataSources:          []string{"CMMS (Maximo/SAP PM)", "IoT Platform", "Spare Parts Inventory", "Production Schedule"},
			ClassificationName:   "Failure Prediction",
			ClassificationDesc:   "AI-predict remaining useful life, classify urgency (routine PM, urgent, emergency), assess production impact",
			PromptTemplate:       "predictive_maintenance",
			DecisioningName:      "Maintenance Scheduling Decision",
			DecisioningDesc:      "Schedule maintenance: next planned downtime (non-urgent), immediate shutdown (safety-critical), or defer with monitoring",
			HITLAfterDecisioning: true,
			HITLSLADuration:      2 * time.Hour,
			ExecutionName:        "Maintenance Work Order Creation",
			ExecutionDesc:        "Create maintenance WO in CMMS, reserve spare parts, schedule technician, coordinate with production for downtime window",
			TargetSystems:        []string{"CMMS", "ERP (spare parts)", "Production Scheduling", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceISO9001},
		ProcessOwner:         "Director of Maintenance / VP Operations",
		PrimaryUsers:         []string{"Reliability Engineer", "Maintenance Planner", "Maintenance Technician", "Production Manager"},
		VolumeEstimate:       "Sensor-triggered, varies by equipment count",
	}
}

// NewMFG04SupplierQuality returns the SOP for Supplier Quality Incoming Inspection.
func NewMFG04SupplierQuality() *sopdomain.SOPDefinition {
	return &sopdomain.SOPDefinition{
		SOPID:       "MFG-04",
		Name:        "Supplier Quality Incoming Inspection and Non-Conformance Management",
		Industry:    sopdomain.IndustryManufacturing,
		Version:     "1.0.0",
		Description: "Inspect incoming materials against specifications, manage non-conformances, track supplier quality performance, and initiate corrective actions.",
		Steps: BuildStandardSteps(StepOverrides{
			IntakeName:           "Receipt Notification",
			IntakeDesc:           "Receive goods receipt notification from warehouse, extract PO, supplier, item, lot/batch, quantity, CoA/CoC documents",
			DataRetrievalName:    "Specification and History Pull",
			DataRetrievalDesc:    "Fetch material specifications, inspection plan, supplier quality history, skip-lot eligibility, AQL sampling plan",
			DataSources:          []string{"ERP (SAP QM)", "Supplier Portal", "QMS", "Incoming Inspection DB"},
			ClassificationName:   "Inspection Result Classification",
			ClassificationDesc:   "Evaluate inspection results against specs: accept, conditional accept, reject, classify non-conformance type",
			PromptTemplate:       "incoming_inspection",
			DecisioningName:      "Disposition Decision",
			DecisioningDesc:      "Determine: accept to stock, return to supplier, use-as-is (with concession), or quarantine pending investigation",
			HITLAfterDecisioning: true,
			HITLSLADuration:      4 * time.Hour,
			ExecutionName:        "Disposition Execution",
			ExecutionDesc:        "Move material to designated status, create NCR if rejected, notify supplier, update supplier scorecard, initiate SCAR if recurring",
			TargetSystems:        []string{"ERP (inventory)", "QMS", "Supplier Portal", "SIEM"},
		}),
		ComplianceFrameworks: []sopdomain.ComplianceFramework{sopdomain.ComplianceISO9001, sopdomain.ComplianceFDACGMP},
		ProcessOwner:         "Director of Supplier Quality / VP Quality",
		PrimaryUsers:         []string{"Incoming Inspector", "Supplier Quality Engineer", "Procurement Manager", "Quality Manager"},
		VolumeEstimate:       "Per-shipment receipt",
	}
}
