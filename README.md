# Nexoraa Platform

Enterprise AI Agent Orchestration Platform — automates 25 SOPs across 6 regulated industries with human-in-the-loop approval gates.

## Industries & SOPs

| Industry | SOPs | Compliance |
|----------|------|------------|
| Financial Services | FS-01 KYC, FS-02 AML, FS-03 Trade Recon, FS-04 Regulatory Reporting | BSA/AML, SOX |
| Insurance | INS-01 FNOL, INS-02 Underwriting, INS-03 Claims, INS-04 Subrogation, CPR-01 Counterparty Risk | SOX |
| Healthcare | HC-01 Prior Auth, HC-02 Medical Coding, HC-03 Eligibility, HC-04 Referral Mgmt | HIPAA |
| Hospital Ops | HOSP-01 Bed Mgmt, HOSP-02 Discharge, HOSP-03 OR Scheduling, HOSP-04 Supply Chain | HIPAA |
| Life Sciences | LS-01 Pharmacovigilance, LS-02 Product Complaints, LS-03 Regulatory Submission, LS-04 Quality CAPA | 21 CFR Part 11, GxP |
| Manufacturing | MFG-01 Work Orders, MFG-02 SPC Quality, MFG-03 Predictive Maint, MFG-04 Supplier Quality | ISO 9001 |

## Architecture

```
                    ┌──────────┐
                    │ React UI │ :3000 (dev) / :3002 (Docker)
                    └────┬─────┘
                         │
                    ┌────▼─────┐        ┌───────────────┐
                    │  API     │ :8080  │  Temporal     │ :7233
                    │  (Gin)   │───────▶│  (Workflows)  │
                    └────┬─────┘        └───────────────┘
                         │
                    ┌────▼─────┐        ┌───────────────┐
                    │  Worker  │ :8081  │  Kafka        │ :9092
                    │ (Outbox) │───────▶│  (Events)     │
                    └────┬─────┘        └───────────────┘
                         │
                    ┌────▼─────┐
                    │ Postgres │ :5432
                    └──────────┘
```

**Three Go binaries:** API server, background worker, Temporal worker — sharing one codebase.

**SOP workflow:** Intake → Data Retrieval → Classification → Decisioning → [HITL Gate] → Execution → Audit

## Quick Start

### Option 1: Full stack in Docker Compose

```bash
# Prerequisites: Docker

# Start the full stack (includes api, worker, temporal-worker, migrations, and UI)
make docker-up

# Test it
curl -X POST http://localhost:8080/api/v2/sops/FS-01/execute \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
  -d '{"payload": {"test": true}}'
```

UI: `http://localhost:3002`

### Option 2: Run Go binaries locally against Docker dependencies

```bash
# Prerequisites: Go 1.25+, Docker, golang-migrate CLI, sqlc

# Start only the supporting services and Dockerized UI
docker compose up -d postgres temporal kafka temporal-ui kafka-ui prometheus grafana ui

# Apply database migrations from the host
make migrate-up

# Run API server
make run-api

# Run background worker in a separate terminal
make run-worker
```

### Frontend

```bash
cd ui && npm install && npm run dev
# Visit http://localhost:3000
```

For the Docker Compose UI, visit `http://localhost:3002`.

## API Endpoints

All v2 endpoints require `X-Tenant-ID` header (any valid UUID for local dev).

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v2/sops/:id/execute` | Start SOP execution |
| GET | `/api/v2/sop-executions/:id` | Get execution status |
| GET | `/api/v2/sop-executions` | List executions (filter: `sop_id`, `status`, `industry`) |
| GET | `/api/v2/hitl/pending` | List pending HITL approvals |
| GET | `/api/v2/hitl/:id` | Get HITL request |
| POST | `/api/v2/hitl/:id/decide` | Approve/reject/escalate |
| GET | `/api/v2/audit/executions/:id` | Get audit trail |
| GET | `/health` | Health check |

## Services (Docker Compose)

| Service | Port | Dashboard |
|---------|------|-----------|
| API | 8080 | — |
| Worker | 8081 | — |
| Temporal | 7233 | http://localhost:8088 |
| Kafka | 9092 | http://localhost:8089 |
| Prometheus | 9091 | http://localhost:9091 |
| Grafana | 3001 | http://localhost:3001 (admin/admin) |
| React UI | 3002 | http://localhost:3002 |
| PostgreSQL | 5432 | `psql postgres://narayana:narayana@localhost:5432/narayana` |

## Testing

```bash
make test-unit          # unit tests
make test-integration   # integration tests (needs Docker)
make test-e2e           # end-to-end (needs full stack)
make test-compliance    # HIPAA, CFR21, BSA/AML, SOX validators
make test-load-sop      # k6 load test (100 concurrent)
```

## Tech Stack

**Backend:** Go 1.25, Gin, PostgreSQL (pgx/v5 + sqlc), Temporal, franz-go (Kafka), zerolog, Prometheus

**Frontend:** React 19, Vite 6, TypeScript, react-router-dom

**Infrastructure:** Docker Compose, Crossplane (AWS multi-cloud), ArgoCD (GitOps), Kustomize (K8s), Backstage (developer portal)

## Project Structure

```
cmd/api/                    HTTP API server
cmd/worker/                 Background job processor
cmd/temporal-worker/        Temporal workflow executor
internal/
  broker/                   Kafka producer/consumer, backpressure, adaptive consumer
  compliance/               HIPAA, CFR21, BSA/AML, SOX validators
  domain/                   Core domain models and errors
  handler/                  HTTP handlers (v1 + v2)
  service/                  Business logic layer
  repository/               Data access (Postgres via sqlc)
  sop/domain/               SOP types (25 definitions)
  sop/registry/             SOP registry (phase files)
  temporal/                 Workflows, activities, signals
  worker/                   Claimer, reaper, publisher, consumer
db/migrations/              SQL migrations
db/queries/                 sqlc query definitions
ui/                         React frontend (Vite)
k8s/                        Kubernetes manifests (base + overlays)
crossplane/                 Multi-cloud compositions + claims
argocd/                     GitOps application definitions
backstage/                  Developer portal catalog
tests/                      E2E, integration, load tests
```

## Feature Flags

| Flag | Default | Description |
|------|---------|-------------|
| `USE_TEMPORAL` | `false` | Enable Temporal workflow execution |
| `EVENT_BUS` | `channel` | Switch outbox transport (`channel` or `kafka`) |

## License

Proprietary — Nexoraa.ai
