# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Quick Start

```bash
make docker-up        # start Postgres, Temporal, Kafka, monitoring
make migrate-up       # apply DB migrations
make run-api          # start API on :8080 (in one terminal)
make run-worker       # start worker on :8081 (in another terminal)
```

Default DB: `postgres://narayana:narayana@localhost:5432/narayana?sslmode=disable`

## Required Tools

- Go 1.25+
- Docker & Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI (`migrate`)
- [sqlc](https://sqlc.dev) (`sqlc generate`)
- [k6](https://k6.io) (load tests only)

## Commands

```bash
# Database
make migrate-down       # rollback migrations
make sqlc-generate      # regenerate Go code from db/queries/*.sql

# Build binaries
make build-api && make build-worker && make build-temporal-worker

# Tests
make test-unit          # go test ./internal/... -v -count=1
make test-integration   # go test ./tests/integration/... (needs Docker — uses testcontainers)
make test-e2e           # go test ./tests/e2e/... (needs full stack running)
make test-compliance    # go test ./internal/compliance/...
make test-load-sop      # k6 run tests/load/sop_load_test.js

# Run a single test
go test ./internal/broker/... -run TestCircuitBreaker -v -count=1
```

## Architecture

Three Go binaries share the same codebase:

- **cmd/api** — HTTP API server (Gin). Serves v1 (generic executions) and v2 (SOP/HITL/Audit) endpoints. Requires `X-Tenant-ID` header on all business routes.
- **cmd/worker** — Background processor. Runs Claimer (picks up pending work), Reaper (reclaims expired leases), Publisher (outbox→channel or Kafka), Consumer (processes events), GaugeCollector (metrics). The `EVENT_BUS` config flag switches between Go channel (`channel`) and Kafka (`kafka`) transport.
- **cmd/temporal-worker** — Registers Temporal workflows and activities, connects to Temporal server. One task queue per industry.

### Dependency Wiring

All dependencies are wired via constructor injection in each `cmd/*/main.go`. The pattern is: `config.Load()` → `pgxpool` → `repository.New*(pool)` → `service.New*(repo, ...)` → `handler.New*(svc)` → register routes. No global state or init() side effects (except Prometheus metric registration via `promauto`).

### Data Flow

```
API Request → Handler → Service → Repository → PostgreSQL
                                    ↓ (transactional outbox)
                              OutboxEvent row
                                    ↓
                              Publisher (polls outbox)
                                    ↓
                         channel (default) OR Kafka
                                    ↓
                              Consumer (processes)
```

For SOP workflows:
```
POST /api/v2/sops/:id/execute
  → SOPService.StartExecution()
    → SOPRegistry.GetByID() (validates SOP exists)
    → SOPRepository.Create() (DB row)
    → TemporalClient.ExecuteWorkflow() (starts 6-step workflow)
      → Intake → DataRetrieval → Classification → Decisioning → [HITL gate] → Execution → Audit
```

### Key Patterns

- **Optimistic locking** — All mutable entities have a `version` column. Updates use `WHERE version = $N` and return `pgx.ErrNoRows` on conflict, mapped to `domain.ErrOptimisticLock`.
- **Transactional outbox** — State mutation + event insertion in a single Postgres TX. Publisher polls `outbox_events WHERE sent = false`, publishes, then marks sent.
- **Tenant isolation** — Every query is scoped by `tenant_id`. The `TenantExtractor` middleware rejects requests without `X-Tenant-ID`.
- **Domain error mapping** — `handler/response.go:mapDomainError()` translates `domain.Err*` types to HTTP status codes. Add new error types there.
- **Service interfaces in handler package** — Handlers define the interface they need (e.g., `SOPServiceInterface`), not the service package. This follows Dependency Inversion and enables testing with mocks.

## Code Conventions

- **Constructor naming** — Use `New<TypeName>()` (e.g., `NewSOPRegistry()`, `NewPostgresSOPRepository(pool)`), not bare `New()`.
- **sqlc nullable types** — Use `pgtype.Text{String: val, Valid: val != ""}` for nullable text, `db.NullSopExecutionStatus{SopExecutionStatus: ..., Valid: true}` for nullable enums.
- **Domain type mapping** — Repository layer converts between `db.*` (sqlc-generated) and `domain.*`/`sopdomain.*` types. Each repo has private `map*()` functions.
- **Temporal client** — `internal/temporal.NewClient(host, namespace, logger)` returns `go.temporal.io/sdk/client.Client`. Pass `nil` when Temporal is disabled.
- **SOP step config access** — Agent step fields: `step.Config.LLMModel` (not `step.LLMModel`), `step.HITLSLADuration` (not `step.SLADuration`), `step.Timeout`.
- **Handler interfaces** — Define service interfaces in the handler package (e.g., `SOPServiceInterface` in `sop_handler.go`), not in the service package.

### API v2 Routes

All require `X-Tenant-ID` header.

| Method | Path | Handler |
|--------|------|---------|
| POST | `/api/v2/sops/:id/execute` | `SOPHandler.StartExecution` |
| GET | `/api/v2/sop-executions/:id` | `SOPHandler.GetExecution` |
| GET | `/api/v2/sop-executions` | `SOPHandler.ListExecutions` (filters: `sop_id`, `status`, `industry`) |
| GET | `/api/v2/hitl/pending` | `HITLHandler.ListPending` |
| GET | `/api/v2/hitl/:id` | `HITLHandler.GetRequest` |
| POST | `/api/v2/hitl/:id/decide` | `HITLHandler.Decide` |
| GET | `/api/v2/audit/executions/:id` | `AuditHandler.GetAuditTrail` |

### Kafka Topic Routing

`broker.TopicForEvent(event)` routes outbox events to Kafka topics using aggregate-type primary routing with event-type prefix fallback:

| AggregateType | Topic |
|---------------|-------|
| `execution`, `sop_execution` | `sop.executions.events` |
| `hitl_request` | `sop.hitl.requests` |
| `hitl_response` | `sop.hitl.responses` |
| `audit_trail` | `audit.trail` |

### SOP Registry

25 SOPs defined across 5 phase files in `internal/sop/registry/`. Each SOP has 6 agent steps (Intake→DataRetrieval→Classification→Decisioning→Execution→Audit), compliance framework tags, and industry classification. The registry is populated at startup via `NewSOPRegistry()`.

### Compliance Validators

`internal/compliance/validator.go` routes to framework-specific validators (HIPAA, CFR21 Part 11, BSA/AML, SOX) based on an SOP's compliance tags. Each validator checks audit entries and HITL requests against framework-specific rules.

## Code Generation

After modifying `db/queries/*.sql`, run `make sqlc-generate`. This regenerates `internal/repository/db/` (models, queries). The sqlc config is in `sqlc.yaml`.

## Docker Compose Services

| Service | Port | Purpose |
|---------|------|---------|
| postgres | 5432 | Database |
| api | 8080 | HTTP API |
| worker | 8081/9090 | Background processor + metrics |
| temporal | 7233 | Workflow orchestration |
| temporal-ui | 8088 | Temporal dashboard |
| temporal-worker | 8082/9093 | Workflow executor + metrics |
| kafka | 9092 | Event streaming (KRaft, no ZooKeeper) |
| kafka-ui | 8089 | Kafka dashboard |
| ui | 3000 | React frontend (Vite + Nginx) |
| prometheus | 9091 | Metrics scraper |
| grafana | 3001 | Dashboards (admin/admin) |

## Feature Flags

| Flag | Values | Default | Effect |
|------|--------|---------|--------|
| `USE_TEMPORAL` | `true`/`false` | `false` | Enables Temporal workflow execution |
| `EVENT_BUS` | `channel`/`kafka` | `channel` | Switches outbox event transport |

## Gotchas

- **Never edit `internal/repository/db/`** — it's sqlc-generated. Edit `db/queries/*.sql` then `make sqlc-generate`.
- **Migrations must run before API/worker** — the binaries assume tables exist. `make migrate-up` after `make docker-up`.
- **`X-Tenant-ID` header required** — all v1/v2 API routes reject requests without it. Use any valid UUID for local dev.
- **Temporal is optional** — `USE_TEMPORAL=false` (default) means SOP executions create DB rows but don't start workflows. Set `USE_TEMPORAL=true` in env or docker-compose to enable.
- **Temporal namespace mismatch** — Config defaults to `nexoraa` but docker-compose `temporal-worker` overrides to `TEMPORAL_NAMESPACE=default` (what `auto-setup` creates). Use `default` for local dev.
- **testcontainers needs Docker** — integration tests (`make test-integration`) spin up real Postgres containers. Docker must be running.
- **Prometheus metrics use `promauto`** — metrics self-register on import. This is the one exception to "no init() side effects."

## Infrastructure

- **Crossplane** (`crossplane/`) — Multi-cloud compositions (PostgreSQL, Kafka, S3, VPC) with dev/staging/prod claims
- **ArgoCD** (`argocd/`) — GitOps app-of-apps pattern, one Application per service
- **K8s** (`k8s/`) — Kustomize base + overlays (dev/staging/prod). Prod has PDBs and HPAs
- **Backstage** (`backstage/`) — Developer portal catalog with all 25 SOPs and service entries
- **Terraform** (`terraform/`) — Legacy AWS ECS/RDS deployment (being replaced by Crossplane)
