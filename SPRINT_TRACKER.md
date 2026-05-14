# Nexoraa Platform — Sprint Tracker

> Last updated: 2026-03-18 | Latest commit: `c274afa` | **ALL SPRINTS COMPLETE**

---

## Overall Progress

| Sprint | Focus | Status | Commit | Files | LOC |
|--------|-------|--------|--------|-------|-----|
| Sprint 1 | SOP Domain + Temporal Foundation | **Done** | `d893acf` | (pre-existing) | — |
| Sprint 2 | Temporal Workflows + Kafka Foundation | **Done** | `43e8459` | 12 | 805 |
| Sprint 3 | API v2 + HITL UI + Remaining SOPs + Outbox→Kafka | **Done** | `ce9a8ff` | 27 | 1,926 |
| Sprint 4 | Crossplane + ArgoCD + Backstage + Broker Hardening | **Done** | `904d4ca` | 45 | 4,291 |
| Sprint 5 | E2E Testing + Compliance + Performance + Production | **Done** | `c274afa` | 20 | 2,240 |
| **Total** | | | | **104** | **~9,262** |

---

## Sprint 1 — SOP Domain Model + Temporal Foundation

### Completed

| Item | Files | Commit | Notes |
|------|-------|--------|-------|
| SOP Domain Model | `internal/sop/domain/sop.go`, `agent_step.go`, `industry.go`, `hitl.go`, `compliance.go`, `audit.go` | `d893acf` | ~18KB total. SOPDefinition, SOPExecution, HITLRequest, AuditEntry, ComplianceFramework types |
| SOP Registry | `internal/sop/registry/registry.go`, `steps.go`, 5 phase files | `d893acf` | 1085 LOC. All 25 SOPs defined across 6 industry phase files |
| DB Migrations | `db/migrations/000011-000013` | `d893acf` | sop_executions, hitl_requests, audit_trail tables |
| DB Queries (sqlc) | `db/queries/sop_executions.sql`, `hitl_requests.sql`, `audit_trail.sql` | `d893acf` | Generated Go code in `repository/db/` |
| Temporal Client | `internal/temporal/client.go` | `d893acf` | Client factory with host/namespace from config |
| Temporal Worker | `internal/temporal/worker.go` | `d893acf` | Worker registration with industry task queues |
| Temporal Logger | `internal/temporal/logger.go` | `d893acf` | Zerolog adapter for Temporal SDK |
| Temporal Worker Binary | `cmd/temporal-worker/main.go` | `d893acf` | Standalone binary |
| Config Extensions | `internal/config/config.go` | `d893acf` | `TEMPORAL_HOST`, `TEMPORAL_NAMESPACE`, `USE_TEMPORAL`, `KAFKA_BROKERS`, `KAFKA_GROUP_ID`, `EVENT_BUS` |
| Docker Compose Temporal | `docker-compose.yml` | `d893acf` | temporalio/auto-setup:1.25.2 + UI:2.31.2 + temporal-worker service |

### Pending

| Item | Planned Files | Why Deferred |
|------|---------------|--------------|
| EKS Terraform | `terraform/eks.tf` | ECS Fargate still running; K8s comes with Sprint 4 (Crossplane/ArgoCD) |
| K8s Namespace Bootstrap | `k8s/base/namespace.yaml` | Same — no K8s cluster yet |

### Design Decisions (Sprint 1)

**D1: SOPs organized by industry phase files, not individual files**
- Plan called for 25 individual files (e.g., `ins_01_fnol.go`, `fs_01_kyc.go`)
- Implemented as 5 phase files (`phase2_financial_services.go`, `phase3_healthcare.go`, etc.) + `registry.go` + `steps.go`
- **Reason:** Reduces file sprawl. Each phase file groups related SOPs from one industry, making it easier to review all SOPs in a vertical together. The registry still supports lookup by ID/industry regardless of file organization.

**D2: EKS/K8s deferred to Sprint 4**
- Plan had EKS bootstrap in Sprint 1
- **Reason:** ECS Fargate is running and stable. No point standing up K8s until we have Crossplane (Sprint 4) to manage it declaratively. Starting K8s now would mean managing it twice — once with Terraform, then again with Crossplane.

---

## Sprint 2 — Temporal Workflows + Kafka Foundation

### Completed

| Item | Files | Commit | Notes |
|------|-------|--------|-------|
| SOP Temporal Workflow | `internal/temporal/workflows/sop_workflow.go` | `d893acf` | 254 LOC. Generic 6-step pipeline with HITL gates, SLA timers, signal handling |
| Workflow Types | `workflows/sop_workflow.go` (types section) | `d893acf` | SOPWorkflowInput, SOPWorkflowOutput, ActivityInput, ActivityOutput, StepConfig |
| Activities | `internal/temporal/activities/activities.go` | `d893acf` | Intake, DataRetrieval, Classification, Decisioning, Execution, Audit, CreateHITLRequest, logAudit |
| HITL Signals | `internal/temporal/signals/hitl.go` | `d893acf` | HITLApproval signal type with decision/decidedBy/reason/timestamp |
| Broker Interfaces | `internal/broker/broker.go` | `43e8459` | EventPublisher + EventConsumer interfaces for transport abstraction |
| Broker Config | `internal/broker/config.go` | `43e8459` | KafkaConfig parsed from app config (brokers, group ID, timeouts) |
| Topic Constants + Manager | `internal/broker/topics.go` | `43e8459` | 4 topics + 4 DLQs, TopicManager (kadm), TopicForEvent router |
| Topic Router Tests | `internal/broker/topics_test.go` | `43e8459` | 16 test cases — aggregate routing, event-type fallback, precedence, defaults |
| Kafka Producer | `internal/broker/producer.go` | `43e8459` | Sync produce with tenant-based partition key, AllISRAcks durability |
| Kafka Consumer | `internal/broker/consumer.go` | `43e8459` | Consumer group with mark+commit pattern, partition assignment logging |
| Kafka Metrics | `internal/broker/metrics.go` | `43e8459` | Prometheus counters/histograms: produced, consumed, errors, latency |
| SOP Event Constants | `internal/domain/sop_event.go` | `43e8459` | 5 aggregate types + 10 event type constants |
| Docker Compose Kafka | `docker-compose.yml` | `43e8459` | Kafka KRaft (cp-kafka:7.7.0, no ZooKeeper) + kafka-ui + env vars on workers |
| Makefile Updates | `Makefile` | `43e8459` | `build-temporal-worker`, `kafka-topics` targets |
| franz-go Dependency | `go.mod`, `go.sum` | `43e8459` | github.com/twmb/franz-go v1.20.7 + kadm v1.17.2 |

### Design Decisions (Sprint 2)

**D3: franz-go over confluent-kafka-go**
- **Reason:** franz-go is pure Go (no CGO, no librdkafka C dependency). This means:
  - Builds with `CGO_ENABLED=0` (our Dockerfile already uses this)
  - No C toolchain needed in CI/CD
  - Simpler cross-compilation for multi-arch images
  - Smaller attack surface
- Trade-off: confluent-kafka-go has more community adoption, but franz-go is production-proven (used at Redpanda, Warpstream) and has excellent API design.

**D4: Synchronous (ProduceSync) over async produce**
- **Reason:** The outbox publisher already batches (fetches N events, publishes, marks sent). If any produce fails, we need to know *which* ones failed to retry them next poll cycle. Async produce with callbacks adds complexity without benefit here.
- `RequiredAcks(AllISRAcks())` ensures records are replicated to all in-sync replicas before ack — strongest durability. The ~5ms extra latency is negligible for a publisher that polls every 2 seconds.

**D5: Aggregate-type routing (primary) + event-type prefix (fallback)**
- **Why not event-type-only?** Couples routing to naming conventions. If someone names an event `custom.foo`, it goes to the default topic silently.
- **Why not fan-out (returning []string topics)?** The Temporal `AuditActivity` writes directly to the `audit_trail` DB table via `logAudit()`. If audit events need Kafka streaming, Sprint 3's service layer will emit them as `AggregateType: "audit_trail"` outbox events. Fan-out at the broker layer would mean double-writing.
- **Why aggregate-type primary?** Matches DDD boundaries already in the code. Each aggregate (execution, sop_execution, hitl_request, hitl_response, audit_trail) maps to exactly one Kafka topic. Forward-compatible — new aggregates get routing rules, new event types within an aggregate don't need changes.
- **Why event-type fallback?** Handles edge cases where `AggregateType` is missing (e.g., legacy events) or generic. Safety net, not the primary path.
- **Default = `sop.executions.events`:** Broadest topic; unknown events land here rather than being silently dropped.

**D6: KRaft mode (no ZooKeeper) for Kafka**
- **Reason:** ZooKeeper is deprecated in Kafka. KRaft is Kafka's built-in Raft consensus — one fewer service to manage. For dev, single-node KRaft (`KAFKA_PROCESS_ROLES: broker,controller`) is simpler. For production (Sprint 5), this extends to multi-node KRaft without architecture changes.

**D7: Mark+Commit pattern for consumer offset management**
- franz-go separates `MarkCommitRecords(record)` (in-memory flag) from `CommitMarkedOffsets(ctx)` (flush to Kafka). Process entire batch, mark each record, commit once. If process crashes mid-batch, uncommitted records get redelivered — combined with the existing `processed_events` dedup table in Postgres, this gives effectively-once semantics without Kafka transactions.

**D8: `EVENT_BUS` feature flag defaults to "channel"**
- **Reason:** Zero behavior change for existing deployments. The Go channel pub/sub continues working exactly as before. Sprint 3C will flip the flag to `"kafka"` and wire the new broker interfaces into the worker main.go. Safe rollout — can flip back to `"channel"` instantly if issues arise.

---

## Sprint 3 — API v2 + HITL UI + Remaining SOPs + Outbox→Kafka Migration

### 3A Completed — API v2 Layer (not yet committed)

| Item | Files | Notes |
|------|-------|-------|
| SOP Repository | `internal/repository/sop_repo.go` | SOPRepository interface + PostgresSOPRepository. Create, GetByID, List (with sop_id/status/industry filters), UpdateStatus, Complete, Fail. Maps sqlc types ↔ domain types. |
| HITL Repository | `internal/repository/hitl_repo.go` | HITLRepository interface + PostgresHITLRepository. GetByID, ListPending, UpdateDecision (optimistic locking), ListByExecution. |
| Audit Repository | `internal/repository/audit_repo.go` | AuditRepository interface + PostgresAuditRepository. ListByExecution with count. |
| SOP Service | `internal/service/sop_service.go` | StartExecution (validates SOP exists in registry → creates DB row → starts Temporal workflow), GetExecution, ListExecutions with pagination. |
| HITL Service | `internal/service/hitl_service.go` | ListPending, Decide (updates DB + signals Temporal workflow to resume). Graceful degradation: if signal fails, decision is still saved. |
| Audit Service | `internal/service/audit_service.go` | GetAuditTrail — returns full audit trail for an execution. |
| SOP Handler | `internal/handler/sop_handler.go` | POST /api/v2/sops/:id/execute, GET /api/v2/sop-executions/:id, GET /api/v2/sop-executions |
| HITL Handler | `internal/handler/hitl_handler.go` | GET /api/v2/hitl/pending, GET /api/v2/hitl/:id, POST /api/v2/hitl/:id/decide |
| Audit Handler | `internal/handler/audit_handler.go` | GET /api/v2/audit/executions/:id |
| API v2 Wiring | `cmd/api/main.go` | v2 route group with TenantExtractor middleware. Optional Temporal client (graceful degradation). |

### API v2 Endpoints (live)

```
POST /api/v2/sops/:id/execute     — Start SOP execution (creates DB row + Temporal workflow)
GET  /api/v2/sop-executions/:id   — Get SOP execution status
GET  /api/v2/sop-executions       — List executions (filter by sop_id, status, industry)
GET  /api/v2/hitl/pending          — List pending HITL requests
GET  /api/v2/hitl/:id              — Get HITL request by ID
POST /api/v2/hitl/:id/decide       — Approve/reject/escalate (sends Temporal signal)
GET  /api/v2/audit/executions/:id  — Get audit trail for an execution
```

### 3A Design Decisions

**D9: Service interfaces defined in handler package, not service package**
- Following the existing pattern (`ExecutionServiceInterface` in `execution_handler.go`). The *consumer* of the interface defines it (Dependency Inversion Principle). This means handlers can be tested with mocks without importing the service package.

**D10: Optional Temporal client in API server**
- The API server creates a Temporal client only if `USE_TEMPORAL=true`. If the client can't connect, it logs a warning and continues — `StartExecution` creates the DB row but skips the workflow. This means the API server is always available even if Temporal is down. The DB is the source of truth; Temporal is the orchestrator.

**D11: HITL Decide is signal-then-forget**
- `Decide()` updates the DB first (atomic, durable), then signals Temporal (best-effort). If the signal fails, the decision is persisted and the workflow will auto-escalate on SLA timeout. This avoids distributed transaction complexity while maintaining correctness.

### 3B Completed — All 25 SOPs Verified

All 25 SOPs already defined across 5 phase files + 1 counterparty risk:
- `phase2_financial_services.go`: FS-01, FS-02, FS-03, FS-04
- `phase2_insurance.go`: INS-01, INS-02, INS-03, INS-04, CPR-01 (Counterparty Risk)
- `phase3_healthcare.go`: HC-01, HC-02, HC-03, HC-04
- `phase3b_hospital_ops.go`: HOSP-01, HOSP-02, HOSP-03, HOSP-04
- `phase4_life_sciences.go`: LS-01, LS-02, LS-03, LS-04
- `phase5_manufacturing.go`: MFG-01, MFG-02, MFG-03, MFG-04

No new files needed — 3B was already complete from Sprint 1.

### 3C Completed — Outbox→Kafka Migration (not yet committed)

| Item | Files | Notes |
|------|-------|-------|
| KafkaPublisher | `internal/worker/kafka_publisher.go` | Polls outbox, publishes to Kafka via `broker.EventPublisher`, uses `TopicForEvent` for routing. Failed publishes skip (retry next cycle). |
| Worker main.go wiring | `cmd/worker/main.go` (modified) | `EVENT_BUS` switch: `"kafka"` → creates producer, ensures topics, starts KafkaPublisher + KafkaConsumer; `"channel"` (default) → existing Go channel path unchanged |
| Admin client helper | `internal/broker/topics.go` (modified) | Added `NewAdminClient()` for topic management |

**D12: KafkaPublisher skips failed events instead of blocking**
- On produce failure, the event stays in the outbox (not marked sent) and retries next poll cycle. This prevents a single bad event from blocking the entire batch. Trade-off: slightly higher latency for failed events, but the publisher never gets stuck.

**D13: Kafka consumer uses simple log handler initially**
- The Kafka consumer logs consumed events rather than routing to the full channel-based Consumer's handler chain. Full handler integration (DLQ, dedup) comes in Sprint 4's broker hardening. This keeps the migration incremental and testable.

### 3D Completed — React UI MVP (not yet committed)

| Item | Files | Notes |
|------|-------|-------|
| Vite + React + TS scaffold | `ui/package.json`, `tsconfig.json`, `vite.config.ts`, `index.html` | Vite 6, React 19, react-router-dom 7. Dev proxy to API on :8080. |
| API client | `ui/src/api/client.ts` | Typed fetch wrapper for all v2 endpoints (SOP, HITL, Audit). Tenant ID header injection. |
| App + Router | `ui/src/main.tsx` | 3-route SPA: Dashboard, HITL Workbench, Analytics. Dark theme nav. |
| Dashboard page | `ui/src/pages/Dashboard.tsx` | SOP execution list with status/industry filters, auto-refresh. |
| HITL Workbench page | `ui/src/pages/HITLWorkbench.tsx` | Pending HITL requests with approve/reject/escalate actions, overdue highlighting. |
| Analytics page | `ui/src/pages/Analytics.tsx` | KPI cards (total, completed, failed, waiting HITL, avg duration), status and industry breakdowns. |
| SOP Execution List | `ui/src/components/SOPExecutionList.tsx` | Sortable table with status badges, duration calculation, monospace IDs. |
| HITL Request Card | `ui/src/components/HITLRequestCard.tsx` | Action card with reason input, overdue border, decision buttons. |
| Docker + Nginx | `ui/Dockerfile`, `ui/nginx.conf` | Multi-stage build (node→nginx), SPA fallback, API proxy to Go backend. |
| Docker Compose | `docker-compose.yml` (modified) | Added `ui` service on port 3000, depends on API. |

**D14: Vite SPA instead of Next.js**
- The plan called for a React client UI. We chose Vite (not Next.js) because: (1) this is a pure client-side SPA that talks to the Go API — no SSR needed; (2) lighter footprint for an internal tool; (3) avoids adding a Node.js server to the production stack. The Nginx container serves the static build and proxies `/api/` to Go.

**D15: Dev default tenant ID for MVP**
- Dashboard hardcodes `TENANT_ID = '00000000-0000-0000-0000-000000000001'` for dev. Production will use auth context. This unblocks UI development without an auth system.

---

## Sprint 4 — Crossplane + ArgoCD + Backstage + Broker Hardening

### Pending

| Item | Planned Files |
|------|---------------|
| **4A: Crossplane** | `crossplane/provider-aws.yaml`, `compositions/`, `claims/` |
| **4B: ArgoCD** | `argocd/bootstrap/`, `applications/`, `k8s/base/`, `k8s/overlays/` |
| **4C: Backstage** | `backstage/app-config.yaml`, `catalog/`, `templates/` |
| **4D: Backpressure** | `internal/broker/backpressure.go` — token bucket per tenant + circuit breaker |
| **4D: Adaptive Consumer** | `internal/broker/adaptive_consumer.go` — spike detection, burst buffer, flipflop cooldown |
| **4D: Consumer Lag Monitor** | `internal/broker/consumer_lag_monitor.go` — Prometheus metrics per topic/partition |

---

## Sprint 5 — E2E Testing + Compliance + Performance + Production

### Pending

| Item | Planned Files |
|------|---------------|
| **5A: E2E Tests** | `tests/e2e/sop_e2e_test.go`, `hitl_e2e_test.go`, `hitl_timeout_test.go`, `multi_tenant_test.go`, `kafka_e2e_test.go`, `compliance_e2e_test.go` |
| **5A: Load Tests** | `tests/load/sop_load_test.js` (k6) |
| **5B: Compliance** | `internal/compliance/hipaa.go`, `cfr21.go`, `bsa_aml.go`, `sox.go`, `validator.go` |
| **5C: Performance Targets** | SOP <30s p95, HITL signal <500ms p99, Kafka delivery <2s, API reads <100ms p95 |
| **5D: K8s Production** | `k8s/overlays/prod/`, `argocd/applications/prod-app-of-apps.yaml`, `crossplane/claims/prod/` |

---

## Dependency Tracking

```
go.mod direct dependencies:
  github.com/gin-gonic/gin v1.11.0
  github.com/google/uuid v1.6.0
  github.com/jackc/pgx/v5 v5.8.0
  github.com/prometheus/client_golang v1.23.2
  github.com/rs/zerolog v1.34.0
  github.com/stretchr/testify v1.11.1
  github.com/testcontainers/testcontainers-go v0.40.0
  github.com/twmb/franz-go v1.20.7          ← Sprint 2
  go.temporal.io/sdk v1.41.1                 ← Sprint 1
```

## Docker Compose Services

| Service | Image | Port | Added |
|---------|-------|------|-------|
| postgres | postgres:15-alpine | 5432 | Original |
| migrate | migrate/migrate:v4.17.0 | — | Original |
| api | custom (Dockerfile) | 8080 | Original |
| worker | custom (Dockerfile) | 8081, 9090 | Original |
| temporal | temporalio/auto-setup:1.25.2 | 7233 | Sprint 1 |
| temporal-ui | temporalio/ui:2.31.2 | 8088 | Sprint 1 |
| temporal-worker | custom (Dockerfile) | 8082, 9093 | Sprint 1 |
| kafka | confluentinc/cp-kafka:7.7.0 | 9092 | Sprint 2 |
| kafka-ui | provectuslabs/kafka-ui | 8089 | Sprint 2 |
| prometheus | prom/prometheus:v2.51.0 | 9091 | Original |
| ui | custom (ui/Dockerfile) | 3000 | Sprint 3 |
| grafana | grafana/grafana:10.4.0 | 3001 | Original |

## Feature Flags

| Flag | Default | Switch To | Sprint |
|------|---------|-----------|--------|
| `USE_TEMPORAL` | `false` | `true` | Sprint 1 (set in temporal-worker compose service) |
| `EVENT_BUS` | `channel` | `kafka` | Sprint 3C (when Outbox→Kafka migration is tested) |
