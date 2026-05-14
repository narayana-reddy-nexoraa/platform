# Nexoraa Platform — Complete Technical Guide

> Enterprise AI Agent Orchestration Platform for Fortune 2000
> Last updated: 2026-03-23 | 112 Go files | 257 total files | ~16,007 Go LOC

---

## Table of Contents

1. [Platform Overview](#1-platform-overview)
2. [Tech Stack](#2-tech-stack)
3. [Architecture & Design Diagrams](#3-architecture--design-diagrams)
4. [What We Built (Sprint Summary)](#4-what-we-built-sprint-summary)
5. [Design Decisions](#5-design-decisions)
6. [Commands Reference](#6-commands-reference)
7. [Connecting to Tools & Dashboards](#7-connecting-to-tools--dashboards)
8. [Important Gaps & Known Issues](#8-important-gaps--known-issues)

---

## 1. Platform Overview

Nexoraa is an enterprise AI platform that automates **25 Standard Operating Procedures (SOPs)** across **6 regulated industries** using AI agent workflows with human-in-the-loop (HITL) approval gates.

**Industries Served:**
- Financial Services (KYC, AML, Trade Recon, Regulatory Reporting)
- Insurance (FNOL, Underwriting, Claims Adjudication, Subrogation, Counterparty Risk)
- Healthcare (Prior Auth, Medical Coding, Eligibility, Referral Mgmt)
- Hospital Operations (Bed Mgmt, Discharge, OR Scheduling, Supply Chain)
- Life Sciences (Pharmacovigilance, Product Complaints, Regulatory Submission, Quality CAPA)
- Manufacturing (Work Orders, SPC Quality, Predictive Maintenance, Supplier Quality)

**Compliance Frameworks:** HIPAA, 21 CFR Part 11, BSA/AML, SOX, GxP, ISO 9001

---

## 2. Tech Stack

### Backend (Go 1.25)

| Component | Technology | Purpose |
|-----------|-----------|---------|
| HTTP Framework | Gin v1.11 | API server (v1 + v2 endpoints) |
| Database | PostgreSQL 15 + pgx/v5 | Primary data store |
| Code Gen | sqlc | Type-safe SQL → Go code generation |
| Workflow Engine | Temporal SDK v1.41 | Durable 6-step SOP workflows |
| Message Broker | franz-go v1.20 (Kafka) | Event streaming, outbox relay |
| Logging | zerolog | Structured JSON logging |
| Metrics | Prometheus client_golang | Counters, histograms, gauges |
| Testing | testify + testcontainers | Unit + integration tests |
| Load Testing | k6 | Performance benchmarks |

### Frontend (React 19)

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Build Tool | Vite 6 | Dev server + production build |
| Framework | React 19 + TypeScript 5.7 | SPA UI |
| Routing | react-router-dom 7 | Client-side routing |
| Serving | Nginx (Alpine) | Static files + API reverse proxy |

### Infrastructure

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Containers | Docker Compose | Local development (11 services) |
| Multi-Cloud IaC | Crossplane | XRDs + Compositions for AWS (RDS, MSK, S3, VPC) |
| GitOps | ArgoCD | App-of-apps pattern, auto-sync |
| Kubernetes | Kustomize | Base + overlays (dev/staging/prod) |
| Developer Portal | Backstage | 25 SOP catalog entries, scaffolder |
| Legacy Deploy | Terraform + ECS Fargate | AWS deployment (being replaced) |
| Monitoring | Prometheus + Grafana | Metrics scraping + dashboards |

### Claude Code Automations

| Type | Tools |
|------|-------|
| MCP Servers | context7, GitHub, Postgres, Docker |
| Hooks | goimports auto-format, go vet, related test runner, sqlc block, migration warn, .env block, Dockerfile warn |
| Skills | `/run-sop-test`, `/new-sop` |
| Subagents | compliance-reviewer |

---

## 3. Architecture & Design Diagrams

### System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENTS                                  │
│  React UI (:3000)  │  API Consumers  │  Backstage (:7007)       │
└────────┬───────────┴────────┬────────┴──────────────────────────┘
         │                    │
         ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                      API SERVER (:8080)                          │
│                                                                  │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │ v1 Routes│  │  v2 Routes   │  │   Health     │              │
│  │Executions│  │SOP/HITL/Audit│  │  /metrics    │              │
│  └────┬─────┘  └──────┬───────┘  └──────────────┘              │
│       │               │                                         │
│  ┌────┴───────────────┴──────┐                                  │
│  │      Middleware Chain      │                                  │
│  │ ErrorHandler → CorrelationID → RequestLogger → TenantExtract │
│  └────────────┬──────────────┘                                  │
│               │                                                  │
│  ┌────────────┴──────────────┐                                  │
│  │     Service Layer          │                                  │
│  │ ExecutionSvc │ SOPSvc      │                                  │
│  │ HITLSvc      │ AuditSvc    │                                  │
│  └────────────┬──────────────┘                                  │
│               │                                                  │
│  ┌────────────┴──────────────┐                                  │
│  │    Repository Layer        │──── sqlc-generated ──── db/     │
│  │ ExecutionRepo │ SOPRepo    │                     queries/*.sql│
│  │ HITLRepo      │ AuditRepo  │                                  │
│  └────────────┬──────────────┘                                  │
└───────────────┼──────────────────────────────────────────────────┘
                │
                ▼
┌───────────────────────┐
│   PostgreSQL (:5432)  │
│                       │
│  executions           │
│  sop_executions       │
│  hitl_requests        │
│  audit_trail          │
│  outbox_events        │
│  dead_letter_queue    │
│  consumer_offsets     │
└───────────┬───────────┘
            │
            │ (transactional outbox)
            ▼
┌───────────────────────────────────────────────────────────────┐
│                      WORKER (:8081)                            │
│                                                                │
│  ┌──────────┐ ┌────────┐ ┌───────────┐ ┌───────────────────┐ │
│  │ Claimer  │ │ Reaper │ │  Gauge    │ │   EVENT_BUS       │ │
│  │(claim    │ │(reclaim│ │ Collector │ │                   │ │
│  │ pending) │ │expired)│ │(metrics)  │ │ channel (default) │ │
│  └──────────┘ └────────┘ └───────────┘ │    OR             │ │
│                                         │ kafka ──────────┐ │ │
│                                         └─────────────────┼─┘ │
└───────────────────────────────────────────────────────────┼───┘
                                                            │
                    ┌───────────────────────────────────────┘
                    ▼
┌───────────────────────────────────────────────────────────────┐
│                    KAFKA (:9092) KRaft                         │
│                                                                │
│  Topics:                                                       │
│  ├── sop.executions.events  (6 partitions)                    │
│  ├── sop.hitl.requests      (3 partitions)                    │
│  ├── sop.hitl.responses     (3 partitions)                    │
│  ├── audit.trail            (6 partitions)                    │
│  └── dlq.*                  (1 partition each)                │
│                                                                │
│  Broker Hardening:                                             │
│  ├── Token Bucket (per-tenant rate limiting)                  │
│  ├── Circuit Breaker (closed → open → half-open)              │
│  ├── Adaptive Consumer (spike detection, burst buffer)        │
│  └── Consumer Lag Monitor (Prometheus gauges)                 │
└───────────────────────────────────────────────────────────────┘
```

### SOP Workflow Execution Flow

```
┌──────────────────────────────────────────────────────────────────────┐
│                    TEMPORAL WORKFLOW ENGINE (:7233)                    │
│                                                                       │
│  POST /api/v2/sops/FS-01/execute                                     │
│       │                                                               │
│       ▼                                                               │
│  ┌─────────┐    ┌──────────────┐    ┌────────────────┐              │
│  │ INTAKE  │───▶│DATA RETRIEVAL│───▶│ CLASSIFICATION │              │
│  │ Parse & │    │ Fetch from   │    │ LLM-based risk │              │
│  │validate │    │ external APIs│    │ scoring        │              │
│  └─────────┘    └──────────────┘    └───────┬────────┘              │
│                                              │                       │
│                                              ▼                       │
│  ┌─────────┐    ┌──────────────┐    ┌────────────────┐              │
│  │  AUDIT  │◀───│  EXECUTION   │◀───│  DECISIONING   │              │
│  │ Write   │    │ Perform      │    │ LLM decision + │              │
│  │ trail   │    │ action       │    │ recommendation │              │
│  └─────────┘    └──────────────┘    └───────┬────────┘              │
│                                              │                       │
│                                     ┌────────┴────────┐             │
│                                     │   HITL GATE?    │             │
│                                     │ (if required)   │             │
│                                     └────────┬────────┘             │
│                                              │                       │
│                              ┌───────────────┼───────────────┐      │
│                              ▼               ▼               ▼      │
│                         ┌─────────┐   ┌──────────┐   ┌──────────┐  │
│                         │APPROVED │   │ REJECTED │   │ TIMEOUT  │  │
│                         │Continue │   │  Stop    │   │Auto-     │  │
│                         │workflow │   │ workflow │   │escalate  │  │
│                         └─────────┘   └──────────┘   └──────────┘  │
│                                                                      │
│  Task Queues (1 per industry):                                       │
│  financial-services-tasks │ insurance-tasks │ healthcare-tasks       │
│  hospital-ops-tasks │ life-sciences-tasks │ manufacturing-tasks      │
└──────────────────────────────────────────────────────────────────────┘
```

### Kafka Topic Routing

```
OutboxEvent
    │
    ├── AggregateType routing (primary)
    │   ├── "execution" | "sop_execution"  ──▶ sop.executions.events
    │   ├── "hitl_request"                 ──▶ sop.hitl.requests
    │   ├── "hitl_response"                ──▶ sop.hitl.responses
    │   └── "audit_trail"                  ──▶ audit.trail
    │
    ├── EventType prefix routing (fallback)
    │   ├── "sop.*" | "execution.*"        ──▶ sop.executions.events
    │   ├── "hitl.request.*"               ──▶ sop.hitl.requests
    │   ├── "hitl.response.*"              ──▶ sop.hitl.responses
    │   └── "audit.*"                      ──▶ audit.trail
    │
    └── Default                            ──▶ sop.executions.events
```

### Transactional Outbox Pattern

```
┌─ Single PostgreSQL Transaction ──────────────────────┐
│                                                       │
│  1. UPDATE executions SET status = 'COMPLETED'        │
│  2. INSERT INTO execution_transitions (...)           │
│  3. INSERT INTO outbox_events (...)                   │
│                                                       │
│  All 3 succeed or all 3 rollback                      │
└───────────────────────────────────────────────────────┘
        │
        │ (Publisher polls every 2s)
        ▼
┌───────────────────┐     ┌───────────────────┐
│ FetchUnsentEvents │────▶│ Publish to Kafka   │
│ (batch of 50)     │     │ (or Go channel)    │
└───────────────────┘     └───────────────────┘
        │
        ▼
┌───────────────────┐
│ MarkEventsSent    │
│ (update sent=true)│
└───────────────────┘
```

### Infrastructure Deployment Model

```
┌─────────────────────────────────────────────────────────┐
│                    Git Repository                        │
│                 (github.com/narayana-reddy-nexoraa/     │
│                  platform.git)                           │
└────────────┬────────────────────────────┬───────────────┘
             │                            │
             ▼                            ▼
┌────────────────────────┐   ┌────────────────────────────┐
│       ArgoCD           │   │       Crossplane           │
│  (GitOps Controller)   │   │  (Multi-Cloud Controller)  │
│                        │   │                            │
│  app-of-apps.yaml      │   │  XRDs:                    │
│  ├── api               │   │  ├── XPostgreSQL (RDS)    │
│  ├── temporal-worker   │   │  ├── XKafkaCluster (MSK)  │
│  ├── kafka (Strimzi)   │   │  ├── XObjectStorage (S3)  │
│  ├── temporal          │   │  └── XNetwork (VPC)       │
│  └── ui               │   │                            │
│                        │   │  Claims:                   │
│  Watches: k8s/overlays │   │  ├── dev/                 │
│  Auto-sync + prune     │   │  ├── staging/             │
│  Drift correction      │   │  └── prod/                │
└────────────┬───────────┘   └────────────┬───────────────┘
             │                            │
             ▼                            ▼
┌─────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                     │
│                                                          │
│  ┌──────────────────────────────────────────────┐       │
│  │              nexoraa-system namespace          │       │
│  │                                                │       │
│  │  Deployments:          Services:               │       │
│  │  ├── api (3 replicas)  ├── api:8080           │       │
│  │  ├── temporal-worker   ├── temporal-worker    │       │
│  │  ├── ui (3 replicas)   ├── ui:80              │       │
│  │  └── kafka (Strimzi)   └── kafka:9092         │       │
│  │                                                │       │
│  │  HPAs: min 3 / max 10 / target CPU 70%        │       │
│  │  PDBs: minAvailable 2 (api, temporal-worker)  │       │
│  └──────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────┘
```

---

## 4. What We Built (Sprint Summary)

### Sprint 1 — SOP Domain + Temporal Foundation (`d893acf`)

| Deliverable | Files | Details |
|-------------|-------|---------|
| SOP Domain Model | 6 files in `internal/sop/domain/` | SOPDefinition, SOPExecution, HITLRequest, AuditEntry, ComplianceFramework, AgentStep |
| SOP Registry | 8 files in `internal/sop/registry/` | 25 SOPs across 5 phase files + registry + steps |
| DB Migrations | 6 files in `db/migrations/` | sop_executions, hitl_requests, audit_trail tables |
| Temporal Integration | 4 files in `internal/temporal/` | Client, worker, logger, cmd/temporal-worker binary |
| Docker Compose | Temporal + Temporal UI services | Ports 7233, 8088 |

### Sprint 2 — Kafka Broker Layer (`43e8459`) — 12 files, 805 LOC

| Deliverable | Files | Details |
|-------------|-------|---------|
| Broker Package | 7 files in `internal/broker/` | EventPublisher/EventConsumer interfaces, KafkaProducer, KafkaConsumer, TopicManager, TopicForEvent router |
| SOP Event Constants | `internal/domain/sop_event.go` | 5 aggregate types + 10 event types |
| Docker Compose | Kafka (KRaft) + Kafka UI | Ports 9092, 8089 |
| Tests | `topics_test.go` | 16 test cases for routing |

### Sprint 3 — API v2 + UI (`ce9a8ff`) — 27 files, 1,926 LOC

| Deliverable | Files | Details |
|-------------|-------|---------|
| Repositories | `sop_repo.go`, `hitl_repo.go`, `audit_repo.go` | sqlc→domain type mapping, optimistic locking |
| Services | `sop_service.go`, `hitl_service.go`, `audit_service.go` | Business logic, Temporal integration |
| Handlers | `sop_handler.go`, `hitl_handler.go`, `audit_handler.go` | 7 API v2 endpoints |
| Outbox→Kafka | `kafka_publisher.go` + worker main.go | EVENT_BUS flag switches transport |
| React UI | 10 files in `ui/` | Dashboard, HITL Workbench, Analytics |

### Sprint 4 — Infrastructure + Broker Hardening (`904d4ca`) — 45 files, 4,291 LOC

| Deliverable | Files | Details |
|-------------|-------|---------|
| Crossplane | 9 YAMLs | AWS provider, 4 XRD+Compositions, dev/staging claims |
| ArgoCD | 6 YAMLs | App-of-apps, 5 service applications |
| Kubernetes | 12 YAMLs | Base manifests + dev/staging overlays |
| Backstage | 12 files | App config, 25 SOP catalog entries, scaffolder |
| Broker Hardening | 5 Go files (751 LOC) | Token bucket, circuit breaker, adaptive consumer, lag monitor |

### Sprint 5 — Testing + Compliance + Production (`c274afa`) — 20 files, 2,240 LOC

| Deliverable | Files | Details |
|-------------|-------|---------|
| E2E Tests | 6 Go test files | SOP lifecycle, HITL, multi-tenant, Kafka, compliance |
| Load Tests | `sop_load_test.js` | k6: 100 concurrent SOP executions |
| Compliance | 6 Go files | HIPAA, CFR21, BSA/AML, SOX validators + tests |
| Production | 6 YAMLs | K8s prod overlay (PDBs, HPAs), prod ArgoCD, prod Crossplane claims |

---

## 5. Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | SOPs grouped by industry phase files, not individual files | Reduces file sprawl; registry lookup by ID works regardless |
| D2 | EKS/K8s deferred to Sprint 4 | ECS Fargate is stable; no point until Crossplane manages it |
| D3 | franz-go over confluent-kafka-go | Pure Go (no CGO), builds with `CGO_ENABLED=0`, simpler cross-compilation |
| D4 | Synchronous ProduceSync over async | Outbox publisher already batches; need to know which events failed |
| D5 | Aggregate-type routing + event-type fallback | Matches DDD boundaries; forward-compatible with new aggregates |
| D6 | KRaft mode (no ZooKeeper) | ZooKeeper deprecated; fewer services to manage |
| D7 | Mark+Commit pattern for consumer offsets | Process batch → mark each → commit once; crash safety via dedup table |
| D8 | EVENT_BUS defaults to "channel" | Zero behavior change for existing deployments; safe rollout |
| D9 | Service interfaces defined in handler package | Dependency Inversion; handlers testable with mocks |
| D10 | Optional Temporal client in API | API always available even if Temporal is down; DB is source of truth |
| D11 | HITL Decide is signal-then-forget | DB update is atomic; Temporal signal is best-effort; workflow auto-escalates on timeout |
| D12 | KafkaPublisher skips failed events | Retries next poll cycle; never blocks the batch |
| D13 | Kafka consumer uses simple log handler initially | Full handler integration (DLQ, dedup) comes in broker hardening |
| D14 | Vite SPA over Next.js | Pure client-side SPA, no SSR needed, lighter footprint |
| D15 | Dev default tenant ID | Unblocks UI dev without auth system |

---

## 6. Commands Reference

### Backend (Go API + Worker)

```bash
# Start all infrastructure
make docker-up

# Apply database migrations (required before running services)
make migrate-up

# Run API server (port 8080)
make run-api

# Run background worker (port 8081 health, 9090 metrics)
make run-worker

# Build all binaries
make build-api && make build-worker && make build-temporal-worker

# Regenerate sqlc code after editing db/queries/*.sql
make sqlc-generate
```

### Frontend (React UI)

```bash
cd ui

# Install dependencies
npm install

# Dev server with hot reload (port 3000, proxies /api to :8080)
npm run dev

# Production build
npm run build

# Preview production build
npm run preview
```

### Testing

```bash
# Unit tests (no external deps needed)
make test-unit

# Integration tests (needs Docker for testcontainers)
make test-integration

# E2E tests (needs full stack running via docker-compose)
make test-e2e

# Compliance validator tests
make test-compliance

# k6 load test (100 concurrent SOP executions)
make test-load-sop

# Run a single test by name
go test ./internal/broker/... -run TestCircuitBreaker -v -count=1
```

### Infrastructure

```bash
# Docker Compose — full stack
make docker-up          # start everything
make docker-down        # stop everything
docker compose logs -f api   # tail API logs

# Kafka
make kafka-topics       # list Kafka topics

# Terraform (legacy AWS)
make tf-plan
make tf-apply
make tf-destroy
```

### API Quick Test

```bash
# Start an SOP execution
curl -X POST http://localhost:8080/api/v2/sops/FS-01/execute \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
  -d '{"payload": {"test": true}}'

# List SOP executions
curl http://localhost:8080/api/v2/sop-executions?limit=10 \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001"

# List pending HITL requests
curl http://localhost:8080/api/v2/hitl/pending \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001"

# Approve an HITL request
curl -X POST http://localhost:8080/api/v2/hitl/{request_id}/decide \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
  -d '{"decision": "APPROVED", "decided_by": "admin", "reason": "looks good"}'

# Get audit trail
curl http://localhost:8080/api/v2/audit/executions/{execution_id} \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001"

# Health check
curl http://localhost:8080/health
```

---

## 7. Connecting to Tools & Dashboards

### Temporal

| Item | URL / Command |
|------|---------------|
| **Temporal UI** | http://localhost:8088 |
| **Temporal Server** | `localhost:7233` (gRPC) |
| **Namespace** | `default` (configured as `nexoraa` in config, but auto-setup uses `default`) |
| **View workflows** | Temporal UI → Workflows → filter by `sop-` prefix |
| **Signal a workflow** | Temporal UI → Workflow → Signal → channel: `hitl-approval` |

```bash
# Check Temporal is running
docker compose logs temporal | tail -5

# List workflows via CLI (if tctl installed)
tctl --address localhost:7233 workflow list
```

### Kafka

| Item | URL / Command |
|------|---------------|
| **Kafka UI** | http://localhost:8089 |
| **Bootstrap Server** | `localhost:9092` (external), `kafka:29092` (internal) |
| **List topics** | `make kafka-topics` |
| **Cluster** | KRaft mode, single broker, cluster ID: `nexoraa-kafka-cluster-001` |

```bash
# List topics
docker compose exec kafka kafka-topics --bootstrap-server localhost:29092 --list

# Consume from a topic
docker compose exec kafka kafka-console-consumer \
  --bootstrap-server localhost:29092 \
  --topic sop.executions.events \
  --from-beginning --max-messages 5

# Describe topic
docker compose exec kafka kafka-topics \
  --bootstrap-server localhost:29092 \
  --describe --topic sop.executions.events
```

### Prometheus

| Item | URL / Command |
|------|---------------|
| **Prometheus UI** | http://localhost:9091 |
| **Config** | `deploy/prometheus.yml` |
| **Scrape targets** | API (:8080/metrics), Worker (:9090/metrics), Temporal Worker (:9093/metrics) |

```bash
# Useful PromQL queries
# SOP execution rate
rate(execution_engine_executions_created_total[5m])

# Kafka produce errors
execution_engine_kafka_produce_errors_total

# Consumer lag per topic
execution_engine_kafka_consumer_lag

# Circuit breaker state
execution_engine_circuit_breaker_state

# Adaptive consumer throttle status
execution_engine_adaptive_throttled
```

### Grafana

| Item | URL / Command |
|------|---------------|
| **Grafana UI** | http://localhost:3001 |
| **Username** | `admin` |
| **Password** | `admin` |
| **Datasource** | Prometheus (auto-provisioned from `deploy/grafana/datasources/`) |
| **Dashboards** | Auto-provisioned from `deploy/grafana/dashboards/` |

### Kafka UI

| Item | URL / Command |
|------|---------------|
| **Kafka UI** | http://localhost:8089 |
| **Cluster name** | `nexoraa-local` |
| **View topics** | Topics → click topic → Messages tab |
| **View consumers** | Consumers → `nexoraa-platform` group |

### Backstage (Developer Portal)

| Item | Details |
|------|---------|
| **Config** | `backstage/app-config.yaml` |
| **Catalog** | 25 SOP entries + 4 service entries |
| **Scaffolder** | `backstage/catalog/templates/new-sop-service.yaml` |
| **Note** | Backstage app itself is not containerized yet — catalog is ready for when it is |

```bash
# To run Backstage locally (when installed):
cd backstage && yarn dev
# Then visit http://localhost:7007
```

### PostgreSQL

| Item | URL / Command |
|------|---------------|
| **Host** | `localhost:5432` |
| **Database** | `narayana` |
| **User** | `narayana` |
| **Password** | `narayana` |

```bash
# Connect via psql
psql postgres://narayana:narayana@localhost:5432/narayana

# Useful queries
SELECT count(*) FROM sop_executions;
SELECT * FROM hitl_requests WHERE decision = 'PENDING' ORDER BY deadline;
SELECT * FROM audit_trail WHERE sop_execution_id = '<id>' ORDER BY created_at;
SELECT * FROM outbox_events WHERE sent = false ORDER BY created_at LIMIT 10;
```

### React UI

| Item | URL / Command |
|------|---------------|
| **Dev server** | http://localhost:3000 (via `cd ui && npm run dev`) |
| **Docker** | http://localhost:3000 (via `docker compose up ui`) |
| **Pages** | Dashboard (`/`), HITL Workbench (`/hitl`), Analytics (`/analytics`) |
| **API proxy** | Dev: Vite proxy to `:8080`, Docker: Nginx proxy to `api:8080` |

---

## 8. Important Gaps & Known Issues

### Resolved Since Last Update (March 23)

| Area | Was | Now |
|------|-----|-----|
| **CI/CD Pipeline** | No `.github/workflows/` | **Done** — CI (lint, test, build, docker, security) + CD (manifest update → ArgoCD) |
| **README.md** | No project README | **Done** — README.md exists |
| **EKS Provisioning** | No K8s cluster | **Done** — `terraform/eks.tf` + `scripts/bootstrap-cluster.sh` |

### Still Missing / Not Yet Implemented

| Area | Gap | Impact | Priority |
|------|-----|--------|----------|
| **Authentication** | No auth system — API uses raw `X-Tenant-ID` header | Anyone can impersonate any tenant | HIGH |
| **LLM Integration** | Activities have `TODO: Implement actual LLM-based classification/decisioning` | SOPs return simulated results | HIGH |
| **UI Auth** | Hardcoded dev tenant ID `00000000-...0001` | Must integrate with auth before production | HIGH |
| **HTTPS/TLS** | All services run HTTP only | Unencrypted in transit | MEDIUM |
| **Secrets Management** | DB password hardcoded in docker-compose and config defaults | Acceptable for dev, needs Vault/KMS for prod | MEDIUM |
| **Kafka Consumer Handlers** | Kafka consumer logs events but doesn't route to full handler chain (DLQ, dedup) | Events consumed but not fully processed via Kafka path | MEDIUM |
| **API Rate Limiting** | Token bucket exists in broker but not wired as API middleware | No ingress-level rate limiting | MEDIUM |
| **Monitoring Alerts** | Prometheus metrics collected but no alert rules configured | No automated alerting on failures | MEDIUM |
| **Backstage Runtime** | Catalog files exist but Backstage app isn't containerized | Developer portal not accessible | LOW |
| **npm install** | `ui/node_modules` not installed — `npm install` required before dev | First-run friction | LOW |

### Features in Code But Not Yet Documented Here

| Feature | Location | Description |
|---------|----------|-------------|
| **BridgeWorkflow** | `internal/temporal/workflows/sop_workflow.go` | Cross-SOP orchestration with dependency ordering (e.g., HOSP-02 Discharge → HOSP-01 Bed Assignment). Uses Temporal ChildWorkflow pattern. |
| **FewShotExamples** | `internal/sop/domain/agent_step.go`, `registry/steps.go` | AgentGoal pattern from Temporal reinsurance case study. Classification and decisioning steps carry domain-specific input/output examples for LLM accuracy. See INS-01 for reference implementation. |
| **UserContextSignal** | `internal/temporal/signals/hitl.go` | Allows humans to inject guidance mid-workflow via a separate signal channel, without being at an HITL gate. Context is merged into the next activity's payload. |
| **MaxContextTokens** | `internal/sop/domain/agent_step.go` | Per-step token limit to prevent context bloat when passing data between activities. Pattern: context isolation from Temporal reinsurance case study. |
| **GitHub Actions CI/CD** | `.github/workflows/ci.yml`, `cd.yml` | CI: lint → unit test → integration test → build → docker push → security scan. CD: update K8s manifests with new image tag → ArgoCD auto-syncs. |
| **EKS Terraform** | `terraform/eks.tf` | EKS cluster + managed node groups + OIDC provider + EBS CSI driver + addons. Reuses existing VPC from `vpc.tf`. |
| **Bootstrap Script** | `scripts/bootstrap-cluster.sh` | One-command cluster provisioning: Terraform → ArgoCD → Crossplane → Strimzi → Temporal → App-of-Apps. Usage: `./scripts/bootstrap-cluster.sh dev` |

### Known Quirks

- **Port 9092** — Kafka uses `:9092` externally. The temporal-worker was remapped to `:9093` for its metrics to avoid conflict.
- **Temporal namespace** — Config says `nexoraa` but auto-setup creates `default`. The temporal-worker docker service overrides to `TEMPORAL_NAMESPACE=default`.
- **sqlc generated code** — Never edit `internal/repository/db/` directly. A PreToolUse hook blocks this, but manual edits outside Claude will be overwritten on next `make sqlc-generate`.
- **Event bus default** — `EVENT_BUS=channel` means Kafka is running but the outbox relay uses Go channels. Flip to `kafka` when ready, but ensure Kafka topics exist first (`TopicManager.EnsureTopics()` runs on worker startup in kafka mode).
- **Counterparty Risk SOP ID** — Registered as `CPR-01` in the code (`internal/sop/registry/phase2_insurance.go`), not a nameless standalone. Use `CPR-01` when calling the API.
- **Temporal health check** — The `docker-compose.yml` uses `$(hostname -i):7233` in the Temporal health check because the server binds to the container's IP, not `0.0.0.0` or `localhost`.
