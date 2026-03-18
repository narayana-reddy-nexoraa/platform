package registry

import (
	"fmt"
	"sync"

	sopdomain "github.com/narayana-platform/execution-engine/internal/sop/domain"
)

// SOPRegistry holds all registered SOP definitions and provides lookup methods.
type SOPRegistry struct {
	mu   sync.RWMutex
	sops map[string]*sopdomain.SOPDefinition
}

// NewSOPRegistry creates a registry pre-loaded with all 25 SOP definitions.
func NewSOPRegistry() *SOPRegistry {
	r := &SOPRegistry{
		sops: make(map[string]*sopdomain.SOPDefinition),
	}
	r.registerAll()
	return r
}

// GetByID returns an SOP definition by its ID (e.g., "FS-01"). Returns error if not found.
func (r *SOPRegistry) GetByID(sopID string) (*sopdomain.SOPDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sop, ok := r.sops[sopID]
	if !ok {
		return nil, fmt.Errorf("SOP not found: %s", sopID)
	}
	return sop, nil
}

// ListByIndustry returns all SOP definitions for a given industry.
func (r *SOPRegistry) ListByIndustry(industry sopdomain.Industry) []*sopdomain.SOPDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*sopdomain.SOPDefinition
	for _, sop := range r.sops {
		if sop.Industry == industry {
			result = append(result, sop)
		}
	}
	return result
}

// ListAll returns all registered SOP definitions.
func (r *SOPRegistry) ListAll() []*sopdomain.SOPDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*sopdomain.SOPDefinition, 0, len(r.sops))
	for _, sop := range r.sops {
		result = append(result, sop)
	}
	return result
}

// Count returns the total number of registered SOPs.
func (r *SOPRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sops)
}

// register adds an SOP definition to the registry.
func (r *SOPRegistry) register(sop *sopdomain.SOPDefinition) {
	r.sops[sop.SOPID] = sop
}

// registerAll loads all 25 SOP definitions.
func (r *SOPRegistry) registerAll() {
	// Phase 2: Financial Services (4 SOPs)
	r.register(NewFS01KYC())
	r.register(NewFS02AML())
	r.register(NewFS03TradeRecon())
	r.register(NewFS04RegulatoryReporting())

	// Phase 2: Insurance (4 SOPs)
	r.register(NewINS01FNOL())
	r.register(NewINS02Underwriting())
	r.register(NewINS03ClaimsAdjudication())
	r.register(NewINS04Subrogation())

	// Phase 2: Standalone
	r.register(NewCounterpartyRisk())

	// Phase 3: Healthcare (4 SOPs)
	r.register(NewHC01PriorAuth())
	r.register(NewHC02MedicalCoding())
	r.register(NewHC03Eligibility())
	r.register(NewHC04ReferralMgmt())

	// Phase 3B: Hospital Operations (4 SOPs)
	r.register(NewHOSP01BedMgmt())
	r.register(NewHOSP02Discharge())
	r.register(NewHOSP03ORScheduling())
	r.register(NewHOSP04SupplyChain())

	// Phase 4: Life Sciences (4 SOPs)
	r.register(NewLS01Pharmacovigilance())
	r.register(NewLS02ProductComplaints())
	r.register(NewLS03RegulatorySubmission())
	r.register(NewLS04QualityCapa())

	// Phase 5: Manufacturing (4 SOPs)
	r.register(NewMFG01WorkOrders())
	r.register(NewMFG02SPCQuality())
	r.register(NewMFG03PredictiveMaint())
	r.register(NewMFG04SupplierQuality())
}
