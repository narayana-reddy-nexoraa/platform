package domain

// Industry represents a business vertical that SOPs serve.
type Industry string

const (
	IndustryFinancialServices Industry = "FINANCIAL_SERVICES"
	IndustryInsurance         Industry = "INSURANCE"
	IndustryHealthcare        Industry = "HEALTHCARE"
	IndustryHospitalOps       Industry = "HOSPITAL_OPS"
	IndustryLifeSciences      Industry = "LIFE_SCIENCES"
	IndustryManufacturing     Industry = "MANUFACTURING"
)

// TaskQueue returns the Temporal task queue name for this industry.
func (i Industry) TaskQueue() string {
	switch i {
	case IndustryFinancialServices:
		return "financial-services-tasks"
	case IndustryInsurance:
		return "insurance-tasks"
	case IndustryHealthcare:
		return "healthcare-tasks"
	case IndustryHospitalOps:
		return "hospital-ops-tasks"
	case IndustryLifeSciences:
		return "life-sciences-tasks"
	case IndustryManufacturing:
		return "manufacturing-tasks"
	default:
		return "default-tasks"
	}
}

// Valid returns true if the industry is a known value.
func (i Industry) Valid() bool {
	switch i {
	case IndustryFinancialServices, IndustryInsurance, IndustryHealthcare,
		IndustryHospitalOps, IndustryLifeSciences, IndustryManufacturing:
		return true
	default:
		return false
	}
}
