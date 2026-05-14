---
name: new-sop
description: Scaffold a new SOP definition with 6 agent steps, register it, and create a Backstage catalog entry. Usage: /new-sop <SOP-ID> <Name> <Industry>
disable-model-invocation: true
---

# New SOP Scaffolder

Create a new Standard Operating Procedure definition with all required boilerplate.

## Arguments

- `$1` — SOP ID (required, e.g., `MFG-05`, `HC-05`)
- `$2` — SOP name (required, e.g., `"Assembly Line Quality Control"`)
- `$3` — Industry (required, one of: `FINANCIAL_SERVICES`, `INSURANCE`, `HEALTHCARE`, `HOSPITAL_OPS`, `LIFE_SCIENCES`, `MANUFACTURING`)

## Steps

### 1. Create SOP Definition

Add the SOP factory function to the appropriate phase file in `internal/sop/registry/`:

| Industry | File |
|----------|------|
| FINANCIAL_SERVICES | `phase2_financial_services.go` |
| INSURANCE | `phase2_insurance.go` |
| HEALTHCARE | `phase3_healthcare.go` |
| HOSPITAL_OPS | `phase3b_hospital_ops.go` |
| LIFE_SCIENCES | `phase4_life_sciences.go` |
| MANUFACTURING | `phase5_manufacturing.go` |

Follow the existing pattern — each SOP factory returns `*sopdomain.SOPDefinition` with:
- SOPID, Name, Industry, Version ("1.0.0"), Description
- 6 AgentSteps: Intake → DataRetrieval → Classification → Decisioning → Execution → Audit
- ComplianceFrameworks (ask user which apply)
- ProcessOwner, PrimaryUsers, VolumeEstimate

### 2. Register in Registry

Add a call to the new factory function in `internal/sop/registry/registry.go` inside the `registerAll()` method, following the existing pattern:
```go
r.register(New{SOPID}{ShortName}())
```

### 3. Create Backstage Catalog Entry

Append to the appropriate industry file in `backstage/catalog/sops/`:
```yaml
---
apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: {sop-id-lowercase}
  title: "{SOP Name}"
  description: "{Description}"
  tags:
    - {industry-lowercase}
    - sop-workflow
  annotations:
    nexoraa.ai/sop-id: "{SOP-ID}"
spec:
  type: sop-workflow
  lifecycle: production
  owner: platform-team
  system: nexoraa-platform
```

### 4. Verify

Run `go build ./internal/sop/...` to confirm the new SOP compiles and is registered.

## Output

Report the SOP ID, name, industry, file modified, and registration status.
