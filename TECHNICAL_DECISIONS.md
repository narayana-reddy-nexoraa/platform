# Nexoraa Platform — Technical Findings, Architecture Decisions & Implementation Guide

**Version:** 2.0
**Date:** March 23, 2026
**Author:** Narayana Chavva
**Context:** Post-IDP code review with Raj (Raja Sekhar). This document consolidates all research, analysis, architecture decisions, and implementation status. **All 5 sprints are complete** — the platform is running locally with 25 SOPs, Temporal, Kafka, CI/CD, and EKS provisioning ready.

---

## Table of Contents

1. [Current State Analysis](#1-current-state-analysis)
2. [Reference Dashboard Analysis](#2-reference-dashboard-analysis)
3. [25 SOP Inventory & Studio Integration](#3-25-sop-inventory--studio-integration)
4. [Temporal — Workflow Orchestration](#4-temporal--workflow-orchestration)
5. [Message Broker — Kafka vs RabbitMQ](#5-message-broker--kafka-vs-rabbitmq)
6. [Spike, Burst & Flipflop Patterns](#6-spike-burst--flipflop-patterns)
7. [Backpressure Mechanisms](#7-backpressure-mechanisms)
8. [Crossplane — Multi-Cloud Infrastructure](#8-crossplane--multi-cloud-infrastructure)
9. [ArgoCD — GitOps Continuous Delivery](#9-argocd--gitops-continuous-delivery)
10. [Backstage — Internal Developer Portal](#10-backstage--internal-developer-portal)
11. [DevOps Stack — How It All Fits Together](#11-devops-stack--how-it-all-fits-together)
12. [Updated Platform Architecture](#12-updated-platform-architecture)
13. [Implementation Roadmap](#13-implementation-roadmap)

---

## 1. Current State Analysis

### 1.1 Platform Codebase (`/platform` Repo)

**Repository:** github.com/narayana-reddy-nexoraa/platform
**Language:** Go 1.25.0
**Architecture:** Monorepo with three binaries (API + Worker + Temporal Worker)

```
platform/
├── cmd/
│   ├── api/main.go              ← HTTP API (Gin, v1+v2 routes, port 8080)
│   ├── worker/main.go           ← Background worker (Claimer, Publisher, Consumer, Reaper)
│   └── temporal-worker/main.go  ← Temporal workflow worker (6 industry task queues)
├── internal/
│   ├── domain/                  ← Core domain (execution, events, SOP events)
│   ├── sop/
│   │   ├── domain/              ← SOP types (SOPDefinition, AgentStep, HITL, Audit, Compliance)
│   │   └── registry/            ← 25 SOP definitions (phase files + registry + steps helper)
│   ├── temporal/
│   │   ├── client.go            ← Temporal client factory + zerolog adapter
│   │   ├── worker.go            ← Worker registration (6 task queues)
│   │   ├── workflows/           ← SOPWorkflow (generic 6-step) + BridgeWorkflow (cross-SOP)
│   │   ├── activities/          ← 7 generic activities (Intake→Audit + CreateHITLRequest)
│   │   └── signals/             ← HITLApproval + UserContextSignal
│   ├── broker/                  ← Kafka (franz-go): producer, consumer, topics, backpressure,
│   │                              adaptive consumer, circuit breaker, lag monitor
│   ├── handler/                 ← HTTP handlers: execution, sop, hitl, audit, health, response
│   ├── service/                 ← Business logic: execution, sop, hitl, audit
│   ├── repository/              ← PostgreSQL repos: execution, sop, hitl, audit
│   ├── compliance/              ← Validators: HIPAA, 21 CFR Part 11, BSA/AML, SOX
│   ├── middleware/              ← Tenant extraction, correlation ID, error handler, request logger
│   ├── worker/                  ← Legacy worker components (Claimer, Publisher, Consumer, Reaper)
│   ├── config/                  ← Config (DB, Temporal, Kafka, feature flags)
│   └── metrics/                 ← Prometheus metrics registration
├── db/
│   ├── migrations/              ← 13 migration files (000001–000013)
│   └── queries/                 ← 9 sqlc query files → generates internal/repository/db/
├── ui/                          ← React 19 SPA (Vite, Dashboard, HITL Workbench, Analytics)
├── terraform/                   ← AWS: ECS Fargate + EKS cluster + RDS + VPC + ALB + ECR
├── k8s/                         ← Kustomize: base manifests + dev/staging/prod overlays
├── crossplane/                  ← Multi-cloud: compositions (PostgreSQL, Kafka, S3, Network) + claims
├── argocd/                      ← GitOps: app-of-apps + per-service Applications
├── backstage/                   ← Developer portal: 25 SOP catalog entries + scaffolder
├── .github/workflows/           ← CI (lint, test, build, docker, security) + CD (manifest update)
├── tests/                       ← E2E, integration, load (k6), compliance tests
├── deploy/                      ← Prometheus + Grafana configs
├── scripts/
│   ├── deploy.sh                ← Legacy AWS deployment
│   └── bootstrap-cluster.sh     ← EKS cluster provisioning (Terraform→ArgoCD→Crossplane→Strimzi→Temporal)
├── docker-compose.yml           ← 11 services: Postgres, API, Worker, Temporal, Kafka, UI, monitoring
├── Dockerfile                   ← Multi-stage: builds api, worker, temporal-worker binaries
└── sqlc.yaml                    ← sqlc code generation config
```

### 1.2 Key Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/gin-gonic/gin` | v1.11.0 | HTTP routing & middleware |
| `github.com/jackc/pgx/v5` | v5.8.0 | PostgreSQL driver (native) |
| `go.temporal.io/sdk` | v1.41.1 | Temporal workflow/activity SDK |
| `github.com/twmb/franz-go` | v1.20.7 | Kafka client (pure Go, no CGO) |
| `github.com/twmb/franz-go/pkg/kadm` | v1.17.2 | Kafka admin (topic management) |
| `github.com/prometheus/client_golang` | v1.23.2 | Metrics exposition |
| `github.com/rs/zerolog` | v1.34.0 | Structured JSON logging |
| `github.com/google/uuid` | v1.6.0 | UUID generation |
| `github.com/stretchr/testify` | v1.11.1 | Test assertions |
| `github.com/testcontainers/testcontainers-go` | v0.40.0 | Integration testing with Docker containers |

### 1.3 Execution Model

```
                    ┌─────────────────────────────────────────┐
                    │          EXECUTION LIFECYCLE              │
                    │                                          │
                    │  CREATED ──▶ CLAIMED ──▶ RUNNING         │
                    │                            │             │
                    │                     ┌──────┴──────┐      │
                    │                     ▼             ▼      │
                    │                 SUCCEEDED      FAILED    │
                    │                                  │       │
                    │                                  ▼       │
                    │                              RETRYING    │
                    │                                          │
                    │  (lease expires) ──▶ RECLAIMED           │
                    └─────────────────────────────────────────┘
```

**Key design patterns already implemented:**

| Pattern | Implementation | Status |
|---------|---------------|--------|
| Multi-tenancy | `tenant_id` on every row + middleware enforcement | Done |
| Idempotency | `idempotency_key` + SHA-256 payload hash | Done |
| Transactional Outbox | Events created atomically in same DB transaction as state change | Done |
| Exactly-once delivery | `processed_events` deduplication table per consumer group | Done |
| Lease-based locking | `locked_by`, `lock_expires_at`, `last_heartbeat_at` fields | Done |
| Dead Letter Queue | `dead_letter_events` table for failed event handlers | Done |
| Graceful shutdown | Context cancellation + WaitGroup drain | Done |
| Optimistic locking | `version` field for concurrent update safety | Done |

### 1.4 Database Schema (PostgreSQL 15, 13 migrations)

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `executions` | Core entity | execution_id, tenant_id, status, payload, idempotency_key, locked_by, attempt_count, max_attempts |
| `outbox_events` | Reliable event publishing | event_id, execution_id, event_type, payload, sent (bool), sequence |
| `processed_events` | Consumer deduplication | event_id + consumer_group (composite PK) |
| `processing_log` | Worker audit trail | log entries per processing action |
| `execution_transitions` | State change history | from_status, to_status, timestamp, reason |
| `consumer_offsets` | Consumer group checkpointing | consumer_group, last_processed_sequence |
| `dead_letter_events` | Failed event storage | event_id, error_reason, retry_count |
| `sop_executions` | SOP workflow instances | sop_execution_id, sop_id, tenant_id, industry, status, temporal_workflow_id, version |
| `hitl_requests` | HITL approval queue | request_id, sop_execution_id, decision, decided_by, deadline, temporal_workflow_id |
| `audit_trail` | Immutable audit entries | audit_id, sop_execution_id, agent_type, input_hash, output_hash, compliance_tags[] |

### 1.5 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/executions` | Create execution (idempotent via `Idempotency-Key` header) |
| `GET` | `/api/v1/executions/:id` | Get execution by ID |
| `GET` | `/api/v1/executions` | List paginated executions (with status filtering) |

**Headers:** `X-Tenant-ID` (required), `Idempotency-Key` (optional for POST)

**API v2 Endpoints (SOP, HITL, Audit):**

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v2/sops/:id/execute` | Start SOP workflow (creates DB record + Temporal workflow) |
| `GET` | `/api/v2/sop-executions/:id` | Get SOP execution by ID |
| `GET` | `/api/v2/sop-executions` | List SOP executions (filters: sop_id, status, industry) |
| `GET` | `/api/v2/hitl/pending` | List pending HITL approval requests |
| `GET` | `/api/v2/hitl/:id` | Get HITL request details |
| `POST` | `/api/v2/hitl/:id/decide` | Approve/reject/escalate HITL request (sends Temporal Signal) |
| `GET` | `/api/v2/audit/executions/:id` | Get audit trail for SOP execution |

### 1.6 Current Infrastructure (AWS-only via Terraform)

| Component | AWS Service | Spec |
|-----------|------------|------|
| Compute | ECS Fargate | CPU: 256, Memory: 512MB per task |
| Database | RDS PostgreSQL 15 | db.t3.micro, 20GB gp3, single-AZ |
| Registry | ECR | Private container registry |
| Load Balancer | ALB | API target group on port 8080 |
| Network | VPC | 10.0.0.0/16, 2 public + 2 private subnets |
| Logging | CloudWatch | Log groups for API + Worker |
| Monitoring | Prometheus + Grafana | Self-hosted via Docker Compose |

### 1.7 Implementation Status (Originally "Gap Analysis")

| Component | Status (March 17) | Status (March 23) |
|-----------|-------------------|-------------------|
| SOP workflow definitions | Not present | **Done** — 25 SOPs in `internal/sop/registry/` |
| Multi-step workflow orchestration | Not present | **Done** — Temporal SOPWorkflow + BridgeWorkflow |
| External message broker | In-process channel only | **Done** — Kafka (franz-go) with adaptive consumer, backpressure |
| Human-in-the-loop gates | Not present | **Done** — HITL via Temporal Signals + API v2 + SLA timers |
| Multi-cloud support | AWS-only Terraform | **Done** — Crossplane compositions + EKS Terraform |
| Client-facing UI | Not present | **Done** — React SPA (Dashboard, HITL Workbench, Analytics) |
| LLM/AI agent integration | Not present | **Stub** — Activities have TODO for actual LLM calls; classification/decisioning return simulated results |
| CI/CD Pipeline | Not present | **Done** — GitHub Actions CI + CD (.github/workflows/) |
| Authentication/Authorization | Header-only tenant extraction | **Unchanged** — still needs auth system before production |
| API Gateway | Not present — direct ALB | **Not needed** — see Appendix B: API Gateway Decision |

---

## 2. Reference Dashboard Analysis

### 2.1 Overview

**URL:** http://34.228.19.10:3000/dashboard
**Platform:** Nexoraa AI Studio (built on Dify open-source LLM platform)
**Logged-in user:** Narayana (narayana.chavva)

### 2.2 Landing Page — 16 Tool Tiles

The dashboard presents a card-based grid layout titled "Our AI Tools & Solutions":

**Row 1 — Core Agent Types:**

| Tile | Description | Links To |
|------|-------------|----------|
| Agent Studio | Build, test, deploy AI agents | `/apps` |
| IRA - Voice Agents | Voice interactions | External: `slagentsdev.slingo.ai` |
| Email Agents | Manage/draft/reason over enterprise emails | Internal |
| Process Agents | Execute/monitor/optimize enterprise workflows | `/apps` |

**Row 2 — Advanced Agents & Tools:**

| Tile | Description |
|------|-------------|
| UI Agents | Perceive/interact with digital interfaces |
| Generative Agents | Context-aware adaptive outcomes |
| Campaign Designer | AI-driven outbound/inbound campaigns |
| QA & Dashboards | Monitor agent KPIs, real-time metrics |

**Row 3 — Studios & Management:**

| Tile | Description | Links To |
|------|-------------|----------|
| Voice Studio | Create/train voice agents with speech synthesis | Internal |
| Knowledge Studio | Curate domain-specific datasets | `/datasets` |
| Prompt Management | Create/version/optimize prompts | Internal |
| Account & User Management | User roles, permissions, access control | Internal |

**Row 4 — Infrastructure:**

| Tile | Description |
|------|-------------|
| Plugin Master | Integrate external APIs/databases/tools |
| Agent Ops | Monitor/orchestrate deployed agents |
| Persona Composer | Design custom AI personas |
| Skill Library | Modular AI skills/task components |

### 2.3 Agent Studio / Workflow Builder (`/apps`)

**Create App Options:**
- Create from Blank
- Create from Template
- Import DSL file
- Import JSON File
- **Create from SOP** (custom feature — Chat mode + Document Upload mode)

**Existing Workflows:**

| Name | Author | Date | Status |
|------|--------|------|--------|
| Hospital Operations Management | Narayana | 03/17/2026 | Unpublished |
| Manufacturing Automation | Admin | 03/16/2026 | Published |
| Banking Credit Ops | Raj | 02/25/2026 | Published |
| Demo | Admin | 02/10/2026 | Published |

### 2.4 Visual Workflow Builder (Dify-based)

The workflow builder is a **drag-and-drop node canvas** with connected nodes:

**Hospital Operations Management** workflow (15+ nodes):

```
START → CASE BOOKING REQUEST IN... → SURGEON PRIVILEGE CHECK (classifier)
  → PRIVILEGE CHECK ESCALATION (branch) → PRIVILEGE CHECK AGGREGATOR
  → AI CASE DURATION PREDICTION → SCHEDULE CONFLICT DETECTION
  → CONFLICT REPORT GENERATOR → BLOCK TIME UTILIZATION M...
  → BLOCK UTILIZATION REPORT → BLOCK RELEASE DECISION
  → OPEN TIME NOTIFICATION → DAILY OR SCHEDULE FINALIZATION
  → FINAL SCHEDULE PUBLICATION → CASE CANCELLATION MANAGEMENT
  → SLOT RECOVERY ACTION → DURATION PREDICTION ACC... → END
```

**Node Types Available:**

| Node Type | Purpose | Example |
|-----------|---------|---------|
| START | Entry point with input variables | `case_booking_req`, `cpt_code`, etc. |
| LLM | AI inference (gpt-4o-mini CHAT) | Case duration prediction |
| Classifier | Route to branches (CLASS 1/2) | Surgeon privilege check |
| Branch | Conditional routing | Escalation vs. proceed |
| Aggregator | Combine variables (ASSIGN VARIABLES) | Merge branch outputs |
| FunctionCalling | External API/tool invocation | System integration calls |
| END | Terminal with output variables | Final output |

**Builder Features:** Test Run, ENV variables, Publish (dropdown), History/versioning, Variable Inspect, Zoom, Undo/Redo

### 2.5 Knowledge Studio (`/datasets`)

- Create from Knowledge Pipeline
- Connect to External Knowledge Base
- External Knowledge API access

### 2.6 Tools & Plugins

**Built-in Tools:** Audio (TTS/ASR), Code Interpreter, CurrentTime, WebScraper, JSON Process, Tavily (search), YahooFinance

**Installed Plugins (8 total):**

| Plugin | Type | Version |
|--------|------|---------|
| Dify Agent Strategies | Agent Strategy | 0.0.32 |
| YahooFinance | Tool | 0.0.5 |
| Tavily | Tool | 0.1.4 |
| JSON Process | Tool | 0.0.2 |
| DeepSeek | Model | 0.0.11 |
| Anthropic | Model | 0.3.3 |
| **SandLogic** | Model (LOCAL) | 0.0.1 |
| OpenAI | Model | 0.2.8 |

### 2.7 IRA Voice Integration

IRA Voice Agents are accessed via external SandLogic platform (`slagentsdev.slingo.ai`):
- Dashboard: Departments (3), Voice Agents (1), Calls, Campaigns
- Client Management: Service, Sales, Default Demo departments
- Voice Agent Detail: Overview, Configuration, Calls, Resources, Prompt versioning, "Try Me!" testing

---

## 3. 25 SOP Inventory & Studio Integration

### 3.1 Complete SOP Library

All 25 SOPs follow a standardized structure from Raja Sekhar's documentation:

**Phase 2 — Financial Services & Insurance (9 SOPs):**

| SOP ID | Name | Volume | Complexity |
|--------|------|--------|------------|
| FS-01 | KYC Customer Onboarding and Due Diligence | 500–5,000 cases/month | Very High |
| FS-02 | AML Transaction Alert Triage | Thousands/day | Very High |
| FS-03 | Trade and Position Reconciliation | Daily | High |
| FS-04 | Regulatory Reporting (DFAST/CCAR/SOX) | Quarterly + ad-hoc | Very High |
| INS-01 | First Notice of Loss (FNOL) Intake and Triage | 5,000–50,000/month | High |
| INS-02 | Underwriting Submission Triage and Intake | Varies | High |
| INS-03 | Claims Adjudication Support | Varies | High |
| INS-04 | Subrogation Identification and Recovery | Varies | Medium |
| — | Counterparty Risk Assessment (standalone) | Per-transaction | Medium |

**Phase 3 — Healthcare (4 SOPs):**

| SOP ID | Name | Volume | Complexity |
|--------|------|--------|------------|
| HC-01 | Prior Authorization Submission and Tracking | 500–10,000/month | High |
| HC-02 | Medical Coding and Claim Edit Resolution | Daily | High |
| HC-03 | Eligibility and Benefits Verification | 10,000+/month | Medium |
| HC-04 | Referral Management and Care Coordination | Daily | Medium |

**Phase 3B — Hospital Operations (4 SOPs):**

| SOP ID | Name | Volume | Complexity |
|--------|------|--------|------------|
| HOSP-01 | Inpatient Bed Management and Patient Flow | 24/7, every 5–30 min | Very High |
| HOSP-02 | Discharge Planning Coordination | Daily | High |
| HOSP-03 | OR Scheduling and Block Time Management | Daily | Very High |
| HOSP-04 | Hospital Supply Chain Replenishment | Daily + alerts | High |

**Phase 4 — Life Sciences (4 SOPs):**

| SOP ID | Name | Volume | Complexity |
|--------|------|--------|------------|
| LS-01 | Pharmacovigilance Case Intake and Triage | 100–50,000+ ICSRs/month | Very High |
| LS-02 | Product Complaint Handling and Triage | Varies | High |
| LS-03 | Regulatory Submission Content Assembly | Per-submission | High |
| LS-04 | Quality Event Triage (CAPA/Deviation) | Varies | High |

**Phase 5 — Manufacturing (4 SOPs):**

| SOP ID | Name | Volume | Complexity |
|--------|------|--------|------------|
| MFG-01 | Production Work Order Management | 500–50,000 active WOs | Very High |
| MFG-02 | Statistical Process Control / Quality Alerts | Real-time sensor data | Very High |
| MFG-03 | Predictive Maintenance Work Order Generation | Sensor-triggered | High |
| MFG-04 | Supplier Quality Incoming Inspection | Per-shipment | High |

### 3.2 Common Multi-Agent Pattern (All 25 SOPs)

Every SOP uses a consistent 6-agent pattern:

```
┌──────────┐   ┌──────────────┐   ┌──────────────┐
│ 1.INTAKE │──▶│ 2.DATA       │──▶│ 3.CLASSIFY/  │
│   AGENT  │   │   RETRIEVAL  │   │   TRIAGE     │
│          │   │   AGENT      │   │   AGENT      │
│ Parse,   │   │ Pull external│   │ Risk score,  │
│ validate │   │ data, APIs   │   │ categorize   │
└──────────┘   └──────────────┘   └──────┬───────┘
                                         │
                              ┌──────────▼────────┐
                              │  HITL GATEWAY      │
                              │  (human approval   │
                              │   if SOP requires) │
                              └──────────┬────────┘
                                         │
┌──────────┐   ┌──────────────┐   ┌──────▼───────┐
│ 6.AUDIT/ │◀──│ 5.EXECUTION  │◀──│ 4.DECISION   │
│  EVIDENCE│   │   AGENT      │   │   AGENT      │
│  AGENT   │   │              │   │              │
│ Immutable│   │ Write to     │   │ Apply rules, │
│ log      │   │ systems      │   │ recommend    │
└──────────┘   └──────────────┘   └──────────────┘
```

### 3.3 SOP-to-Studio Workflow Mapping

Each SOP translates to a Dify-style workflow in the studio:

| SOP Element | Studio Node Type |
|-------------|-----------------|
| Intake (parse input) | START node + LLM node (extraction) |
| Data Retrieval (API calls) | FunctionCalling node (HTTP tool) |
| Classification/Triage | Classifier node (CLASS 1/2/3 branches) |
| HITL Gate | Branch node + external webhook (pause for approval) |
| Decisioning (LLM reasoning) | LLM node (gpt-4o-mini or Claude) |
| Execution (system writes) | FunctionCalling node (API tool) |
| Audit logging | Aggregator node + END node |

### 3.4 Industry-Specific Compliance Requirements

| Industry | Key Regulations | Human-in-the-Loop Gates |
|----------|----------------|------------------------|
| Financial Services | BSA/AML, OFAC, SOX, DFAST/CCAR | All Medium/High risk decisions, SAR filing |
| Insurance | State DOI, NAIC | High-severity claims, fraud-flagged, subrogation |
| Healthcare | HIPAA, CMS PA API | Clinical necessity, PHI transmission |
| Hospital Ops | CMS Conditions of Participation | Bed assignments, patient movement, discharge |
| Life Sciences | 21 CFR Part 11, GxP, GVP, ICH E2B | All E2B submissions (medical reviewer e-signature) |
| Manufacturing | FDA cGMP, ISO 9001 | Work order release, quality holds, CAPA |

---

## 4. Temporal — Workflow Orchestration

### 4.1 What Is Temporal?

Temporal is a **durable workflow orchestration engine**. You define workflows as code (Go, Java, TypeScript), and Temporal guarantees execution survives crashes, restarts, deployments, and network failures.

Key concepts:

| Concept | Description |
|---------|-------------|
| **Workflow** | A function that orchestrates activities. Has durable state — survives crashes. |
| **Activity** | A single unit of work (API call, DB write, LLM inference). Can fail and be retried. |
| **Task Queue** | Workers poll task queues for work. Replaces your Claimer polling loop. |
| **Signal** | External input to a running workflow. Perfect for HITL (human sends approval signal). |
| **Query** | Read-only access to workflow state. Check "where is this SOP execution right now?" |
| **Timer** | Sleep for arbitrary duration inside a workflow. SLA deadline enforcement. |
| **Child Workflow** | Sub-workflows. One SOP can spawn sub-SOPs. |
| **Saga** | Compensating transactions — undo step 3 if step 5 fails. |

### 4.2 Why Temporal IS Suitable for the 25 SOPs

| Requirement from SOPs | Temporal Capability |
|----------------------|---------------------|
| Multi-step workflows (30-50 steps per SOP) | Workflows are coded as sequential/parallel steps |
| Human-in-the-loop (every SOP has mandatory gates) | **Signals** — workflow pauses, waits for human input, resumes (can wait for days/weeks) |
| SLA enforcement (7-day PV deadlines, 15-min FNOL triage) | **Timers** — fire alerts before deadline, auto-escalate on breach |
| Retry with backoff (API failures, transient errors) | **Retry Policies** — declarative, exponential backoff, max attempts |
| Long-running processes (PV cases take days, KYC reviews take weeks) | Workflows survive indefinitely — no timeout issues |
| Audit trail (who did what, when, why) | **Event History** — every step recorded with timestamps |
| Versioning (deploy new SOP versions safely) | **Workflow Versioning** — old instances continue on old code, new instances use new code |
| Observability (track every execution's progress) | **Temporal Web UI** — real-time visibility into all running workflows |
| Saga/compensation (undo if downstream fails) | **Compensation pattern** — built-in |

### 4.3 How Temporal Maps to Current Code

| Current Go Pattern | Temporal Replacement |
|-------------------|---------------------|
| `claimer.go` — polls DB for CREATED executions | **Task Queue** — Temporal automatically dispatches to workers |
| `publisher.go` — polls outbox for unsent events | **Eliminated** — Temporal manages event flow internally |
| `consumer.go` — processes events from channel | **Activities** — each handler is a Temporal activity |
| `reaper.go` — reclaims expired leases | **Eliminated** — Temporal handles timeouts and heartbeats natively |
| `execution.go` — status state machine | **Workflow State** — Temporal manages state transitions |
| `outbox_events` table | **Eliminated** — Temporal provides guaranteed delivery |
| `dead_letter_events` table | **Failed Activities** — visible in Temporal UI, auto-retried |
| `execution_transitions` table | **Event History** — built into Temporal, queryable |

### 4.4 Actual Implementation: Generic SOPWorkflow (Go)

> **Note:** The original design proposed per-SOP workflow functions (e.g., `FNOLWorkflow`, `KYCWorkflow`).
> The actual implementation uses a **single generic `SOPWorkflow`** that handles all 25 SOPs via configuration.
> This reduces code duplication — SOPs differ by config (steps, prompts, data sources), not by workflow code.

The code is in `internal/temporal/workflows/sop_workflow.go`. Key patterns adopted from the Temporal reinsurance case study:

1. **Generic SOPWorkflow** — iterates through SOP-defined steps, executes activities by step type
2. **BridgeWorkflow** — orchestrates multiple related SOPs with dependency ordering (e.g., HOSP-02 Discharge → HOSP-01 Bed Assignment)
3. **HITL via Signals** — workflow pauses at HITL gates, waits for human signal or SLA timeout → auto-escalates
4. **UserContextSignal** — humans can inject guidance mid-workflow without being at an HITL gate
5. **FewShotExamples** — classification and decisioning steps carry domain-specific input/output examples for LLM accuracy
6. **MaxContextTokens** — prevents token bloat when passing data between steps

```go
// Simplified flow (actual code in internal/temporal/workflows/sop_workflow.go):
func SOPWorkflow(ctx workflow.Context, input SOPWorkflowInput) (*SOPWorkflowOutput, error) {
    for _, step := range input.Steps {
        // Execute activity by step type (Intake, DataRetrieval, Classification, etc.)
        activityName := activityForStepType(step.StepType)
        actInput := ActivityInput{
            SOPExecutionID: input.SOPExecutionID,
            SOPID:          input.SOPID,
            Payload:        lastOutput,
            StepConfig:     step,         // carries LLMModel, FewShotExamples, MaxContextTokens
            UserContext:     pendingCtx,   // human-injected guidance if any
        }
        workflow.ExecuteActivity(actCtx, activityName, actInput).Get(ctx, &actOutput)

        // HITL gate (if step requires it)
        if step.HITLRequired {
            workflow.ExecuteActivity(actCtx, "CreateHITLRequest", hitlInput) // DB record
            signalCh := workflow.GetSignalChannel(ctx, "hitl-approval")
            // Wait for: human approval signal OR SLA timer → auto-escalate
        }
    }
}
```

**7 registered activities** (generic, shared by all 25 SOPs):
- `Intake` — parse/validate input payload
- `DataRetrieval` — fetch from external systems (TODO: actual API calls)
- `Classification` — LLM-based categorization/risk scoring (TODO: actual LLM integration)
- `Decisioning` — LLM-based decision/recommendation (TODO: actual LLM integration)
- `CreateHITLRequest` — create DB record + get Temporal workflow identity for signal routing
- `Execution` — write to target systems (TODO: actual system writes)
- `Audit` — insert immutable audit trail entry with SHA-256 hashes

### 4.5 Temporal Integration Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                                                                │
│  Go API Service (Gin)                                          │
│  POST /api/v2/sops/INS-01/execute                             │
│       │                                                        │
│       ▼                                                        │
│  Temporal Client → Starts SOPWorkflow on "insurance-tasks"     │
│                                                                │
│  POST /api/v2/hitl/:id/decide                                 │
│       │                                                        │
│       ▼                                                        │
│  Temporal Client → Sends HITLApproval Signal to workflow       │
│                                                                │
│  GET /api/v2/sop-executions/:id                                │
│       │                                                        │
│       ▼                                                        │
│  SOPRepository → Queries DB (Temporal state via workflow ID)   │
│                                                                │
└────────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌────────────────────────────────────────────────────────────────┐
│  Temporal Server (self-hosted or Temporal Cloud)                │
│                                                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐ │
│  │ Workflow      │  │ Matching     │  │ History Service      │ │
│  │ Service       │  │ Service      │  │ (event log)          │ │
│  └──────────────┘  └──────────────┘  └──────────────────────┘ │
│                                                                │
│  Backend: PostgreSQL (reuses your existing RDS)                │
│  or Temporal Cloud (managed, recommended for production)       │
└────────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌────────────────────────────────────────────────────────────────┐
│  Go Worker Service (Temporal Worker)                           │
│                                                                │
│  Registers:                                                    │
│  ├── Workflows: SOPWorkflow (generic), BridgeWorkflow (cross-SOP) │
│  └── Activities (7 generic, shared by all 25 SOPs):            │
│       ├── Intake              (parse/validate input)           │
│       ├── DataRetrieval       (fetch external data)            │
│       ├── Classification      (LLM risk scoring)              │
│       ├── Decisioning         (LLM decision/recommendation)   │
│       ├── CreateHITLRequest   (DB record + signal routing)    │
│       ├── Execution           (write to target systems)       │
│       └── Audit               (immutable audit trail)         │
│                                                                │
│  Task Queues:                                                  │
│  ├── "financial-services-tasks" (FS-01 to FS-04, CPR-01)      │
│  ├── "insurance-tasks" (INS-01 to INS-04)                      │
│  ├── "healthcare-tasks" (HC-01 to HC-04)                       │
│  ├── "hospital-ops-tasks" (HOSP-01 to HOSP-04)                 │
│  ├── "life-sciences-tasks" (LS-01 to LS-04)                    │
│  └── "manufacturing-tasks" (MFG-01 to MFG-04)                  │
└────────────────────────────────────────────────────────────────┘
```

### 4.6 Decision: Temporal Deployment Model

| Option | Pros | Cons | Recommendation |
|--------|------|------|---------------|
| **Self-hosted** (Docker/K8s) | Full control, no vendor dependency, uses existing RDS | Operational overhead (upgrades, scaling, HA) | Good for dev/staging |
| **Temporal Cloud** (managed) | Zero ops, auto-scaling, HA, enterprise support | Monthly cost, data leaves your infra | **Recommended for production** |

---

## 5. Message Broker — Kafka vs RabbitMQ

### 5.1 Decision: Temporal + Kafka (Complementary Roles)

**Temporal = Workflow Control Plane** (orchestrates SOP steps)
**Kafka = Event Data Plane** (streams high-volume events between services)

They are NOT competitors — they serve different purposes:

| Concern | Temporal | Kafka |
|---------|----------|-------|
| "Execute step 3 of KYC workflow" | Yes | No |
| "Stream 10,000 ADT events per minute from EHR" | No | Yes |
| "Wait for human approval for 3 days" | Yes (Signals) | No |
| "Fan out audit events to 5 consumers" | No | Yes (consumer groups) |
| "Retry a failed API call with backoff" | Yes (Activity retries) | Partial (DLQ + retry topics) |

### 5.2 Kafka vs RabbitMQ Analysis

| Factor | Kafka | RabbitMQ | Winner |
|--------|-------|----------|--------|
| **Event ordering** | Partition-level ordering guaranteed | Queue-level ordering, complex with competing consumers | **Kafka** |
| **Event replay** | Full log-based replay from any offset | Messages consumed and gone (unless DLQ) | **Kafka** |
| **Throughput** | Millions/sec per cluster | Tens of thousands/sec | **Kafka** |
| **Durability** | Replicated log, configurable retention | Can persist but designed for transient delivery | **Kafka** |
| **Stream processing** | Native (Kafka Streams, ksqlDB, Flink) | Not designed for it | **Kafka** |
| **Ecosystem** | Schema Registry, Connect, MirrorMaker, ksqlDB | Plugins, but smaller | **Kafka** |
| **Operational complexity** | Higher (KRaft cluster, partitions, consumer groups, topic config) | Lower, simpler to deploy and manage | **RabbitMQ** |
| **Latency** | Low (ms), but not ultra-low | Ultra-low (sub-ms for small messages) | **RabbitMQ** |
| **Routing flexibility** | Topic-based, limited | Exchange/binding patterns (direct, topic, fanout, headers) | **RabbitMQ** |
| **Protocol support** | Custom binary protocol | AMQP 0.9.1 (standard), MQTT, STOMP | **RabbitMQ** |

### 5.3 Decision: Kafka

**Why Kafka for Nexoraa:**

1. **You already built an outbox pattern** — Kafka IS the natural evolution of your in-process channel. Your `outbox_events` table → Kafka Connect CDC → Kafka topic.
2. **Event replay for audit** — regulated industries require reconstructing what happened. Kafka's log retention gives you this.
3. **Real-time event streams** — HOSP-01 (HL7 ADT events every 5-30 min), MFG-02 (sensor data real-time), FS-02 (transaction alerts).
4. **Consumer groups** — different SOP workflows consume from the same event stream independently.
5. **Stream processing** — can run Kafka Streams or Flink for complex event processing (e.g., detecting transaction patterns for AML).

### 5.4 Kafka Topic Design

| Topic | Producers | Consumers | Purpose |
|-------|-----------|-----------|---------|
| `sop.executions.events` | API Service (via outbox) | Audit Service, Analytics, SIEM | All SOP execution events |
| `sop.hitl.requests` | Temporal Activities | HITL UI Service | Human approval requests |
| `sop.hitl.responses` | HITL UI Service | Temporal Activities (via signal) | Human decisions |
| `integrations.ehr.adt` | EHR Connector | HOSP-01 to HOSP-04 workflows | Hospital ADT events |
| `integrations.erp.events` | ERP Connector | MFG-01 to MFG-04 workflows | Manufacturing events |
| `integrations.screening.results` | Screening APIs | FS-01, FS-02 workflows | KYC/AML screening results |
| `audit.trail` | All services | Audit Store (OpenSearch), Compliance | Immutable audit events |
| `dlq.{original-topic}` | Kafka consumer error handler | DLQ processor | Dead letter events |

---

## 6. Spike, Burst & Flipflop Patterns

### 6.1 Definitions

These are **traffic patterns** that stress message processing systems:

**Spike** — Sudden, short-lived surge in volume.
```
Normal:    ████████
Spike:     ████████████████████████████████  (3-10x volume for minutes)
Normal:    ████████
```
- **Example:** Natural disaster triggers thousands of FNOL claims in minutes (INS-01)
- **Example:** Market event triggers mass AML alerts (FS-02)

**Burst** — Sustained high throughput for an extended period.
```
Normal:    ████████
Burst:     ████████████████████  (2-3x volume for hours)
Normal:    ████████
```
- **Example:** Month-end regulatory reporting (FS-04 DFAST/CCAR)
- **Example:** Quarterly PV reporting deadlines (LS-01)
- **Example:** MRP nightly run generates 50,000 work orders (MFG-01)

**Flipflop** — Oscillating high/low patterns that confuse auto-scalers.
```
High:      ████████████████████████
Low:       ████████
High:      ████████████████████████
Low:       ████████
(repeats)
```
- **Example:** Hospital bed management — morning admission surge, afternoon lull, evening surge (HOSP-01)
- **Example:** Trading hours vs. after-hours for financial services

### 6.2 Handling Strategies

| Pattern | Strategy | Implementation |
|---------|----------|---------------|
| **Spike** | Absorb + Auto-scale | Kafka partitions buffer messages; Kubernetes HPA scales consumers based on consumer lag metric |
| **Spike** | Load shedding | Drop low-priority messages when queue depth exceeds threshold; preserve high-priority SOPs |
| **Burst** | Pre-provision | Schedule extra capacity ahead of known bursts (month-end, quarterly deadlines) |
| **Burst** | Priority queues | Dedicated Kafka consumer groups for critical SOPs; lower-priority SOPs share remaining capacity |
| **Flipflop** | Cooldown period | Auto-scaler waits N minutes after last scale action before acting again; prevents oscillation |
| **Flipflop** | Predictive scaling | Use historical patterns (HOSP-01 always surges at 06:00 and 18:00) to pre-scale |
| **Flipflop** | Min/max bounds | Set minimum and maximum consumer count to bound oscillation range |

### 6.3 Implementation in Go

```go
// internal/broker/adaptive_consumer.go

type AdaptiveConsumerConfig struct {
    MinWorkers         int           // Baseline workers (flipflop low floor)
    MaxWorkers         int           // Ceiling workers (spike/burst maximum)
    ScaleUpThreshold   int64         // Queue depth to trigger scale-up
    ScaleDownThreshold int64         // Queue depth to trigger scale-down
    CooldownPeriod     time.Duration // Prevent flipflop oscillation
    ScaleStep          int           // Workers to add/remove per scaling event
}

type AdaptiveConsumer struct {
    config          AdaptiveConsumerConfig
    currentWorkers  int
    lastScaleAction time.Time
    queueDepthGauge prometheus.Gauge // Observable metric
    workerGauge     prometheus.Gauge // Observable metric
}

func (ac *AdaptiveConsumer) EvaluateScaling(currentLag int64) ScaleDecision {
    now := time.Now()

    // Cooldown guard — prevents flipflop oscillation
    if now.Sub(ac.lastScaleAction) < ac.config.CooldownPeriod {
        return ScaleDecision{Action: "hold", Reason: "cooldown active"}
    }

    if currentLag > ac.config.ScaleUpThreshold && ac.currentWorkers < ac.config.MaxWorkers {
        return ScaleDecision{Action: "scale_up", NewCount: min(ac.currentWorkers + ac.config.ScaleStep, ac.config.MaxWorkers)}
    }

    if currentLag < ac.config.ScaleDownThreshold && ac.currentWorkers > ac.config.MinWorkers {
        return ScaleDecision{Action: "scale_down", NewCount: max(ac.currentWorkers - ac.config.ScaleStep, ac.config.MinWorkers)}
    }

    return ScaleDecision{Action: "hold", Reason: "within thresholds"}
}
```

### 6.4 Kafka-Specific Configuration for Spike/Burst

```properties
# Producer — handle spikes by buffering
linger.ms=5                    # Batch messages for 5ms before sending (absorbs micro-spikes)
batch.size=16384               # Max batch size (16KB)
buffer.memory=33554432         # 32MB producer buffer (absorbs larger spikes)
max.block.ms=60000             # Block up to 60s if buffer is full (backpressure)

# Consumer — handle bursts with tunable fetch
max.poll.records=500           # Process 500 records per poll (burst throughput)
fetch.min.bytes=1024           # Wait for 1KB before fetching (reduce idle polls)
fetch.max.wait.ms=500          # Max wait for fetch.min.bytes (latency tradeoff)
session.timeout.ms=30000       # Detect dead consumers in 30s
heartbeat.interval.ms=10000    # Consumer heartbeat frequency

# Topic — handle spikes with partitions
num.partitions=12              # 12 partitions = up to 12 parallel consumers
retention.ms=604800000         # 7-day retention for replay/audit
```

---

## 7. Backpressure Mechanisms

### 7.1 What Is Backpressure?

Backpressure occurs when a system receives data **faster than it can process it**. Without backpressure controls, the system will:
1. Buffer messages in memory → OOM crash
2. Drop messages silently → data loss
3. Cascade failures to upstream services

```
WITHOUT backpressure:
Producer (10K/sec) → [Queue grows unbounded] → Consumer (5K/sec) → CRASH

WITH backpressure:
Producer (10K/sec) → [Bounded queue → FULL signal] → Producer slows to 5K/sec → STABLE
```

### 7.2 Backpressure Layers for Nexoraa

```
┌─────────────────────────────────────────────────────────────┐
│                 BACKPRESSURE LAYERS                           │
│                                                              │
│  Layer 1: API INGRESS                                        │
│  ├── Rate Limiting (429 Too Many Requests)                   │
│  ├── Per-tenant quotas                                       │
│  └── Circuit breaker on API gateway                          │
│                                                              │
│  Layer 2: MESSAGE BROKER (Kafka)                             │
│  ├── Bounded producer buffer (buffer.memory=32MB)            │
│  ├── max.block.ms=60s (producer blocks when buffer full)     │
│  ├── Consumer lag monitoring (Prometheus + alert)             │
│  └── Auto-scaling consumers via Kubernetes HPA               │
│                                                              │
│  Layer 3: TEMPORAL WORKFLOWS                                 │
│  ├── Task queue rate limiting (per namespace)                │
│  ├── Worker max concurrent activities                        │
│  └── Activity retry backoff (exponential)                    │
│                                                              │
│  Layer 4: INTEGRATION HUB (downstream systems)               │
│  ├── Circuit breaker per connector                           │
│  │   (if EHR/ERP is slow, stop sending requests)             │
│  ├── Bulkhead pattern                                        │
│  │   (isolate connector failures — EHR down ≠ ERP down)      │
│  └── Retry with exponential backoff + jitter                 │
│                                                              │
│  Layer 5: LOAD SHEDDING (last resort)                        │
│  ├── Priority-based: drop low-priority work first            │
│  ├── Tenant-based: enforce per-tenant throughput limits       │
│  └── SOP-based: critical SOPs (PV, AML) never shed           │
└─────────────────────────────────────────────────────────────┘
```

### 7.3 Implementation in Go

```go
// internal/middleware/backpressure.go

// Layer 1: API Rate Limiter
type TenantRateLimiter struct {
    limiters map[string]*rate.Limiter  // per-tenant
    mu       sync.RWMutex
    defaultRPS int                     // requests per second
}

func (rl *TenantRateLimiter) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.GetHeader("X-Tenant-ID")
        limiter := rl.getOrCreate(tenantID)
        if !limiter.Allow() {
            c.JSON(429, gin.H{"error": "rate limit exceeded", "retry_after_seconds": 1})
            c.Abort()
            return
        }
        c.Next()
    }
}

// Layer 4: Circuit Breaker for Integration Connectors
type CircuitBreaker struct {
    name           string
    state          State  // CLOSED, OPEN, HALF_OPEN
    failureCount   int
    failureThreshold int  // failures before opening
    resetTimeout   time.Duration
    lastFailure    time.Time
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    if cb.state == OPEN {
        if time.Since(cb.lastFailure) > cb.resetTimeout {
            cb.state = HALF_OPEN // try one request
        } else {
            return ErrCircuitOpen
        }
    }
    err := fn()
    if err != nil {
        cb.failureCount++
        cb.lastFailure = time.Now()
        if cb.failureCount >= cb.failureThreshold {
            cb.state = OPEN
        }
        return err
    }
    cb.state = CLOSED
    cb.failureCount = 0
    return nil
}
```

### 7.4 Monitoring Backpressure

| Metric | Threshold | Action |
|--------|-----------|--------|
| Kafka consumer lag (messages behind) | > 10,000 | Scale up consumers |
| Kafka consumer lag (messages behind) | > 100,000 | Alert on-call + load shed low-priority |
| API 429 rate | > 5% of requests | Alert — investigate spike source |
| Circuit breaker OPEN count | > 0 | Alert — downstream system degraded |
| Temporal activity schedule-to-start latency | > 30s | Scale up workers |
| Pod memory usage | > 80% | Scale up pods / investigate leak |

---

## 8. Crossplane — Multi-Cloud Infrastructure

### 8.1 What Is Crossplane?

Crossplane is a **Kubernetes-native cloud infrastructure management** tool. Instead of writing Terraform HCL, you define cloud resources as **Kubernetes Custom Resources (CRDs)**.

### 8.2 Why Crossplane (Replaces Terraform for Multi-Cloud)

| Feature | Terraform (Current) | Crossplane (Target) |
|---------|---------------------|---------------------|
| Language | HCL (HashiCorp) | YAML (Kubernetes-native) |
| State management | `.tfstate` file (S3 backend) | Kubernetes etcd (no separate state) |
| Execution model | `plan` → `apply` (manual or CI) | **Continuous reconciliation** (Kubernetes controller) |
| Drift detection | Manual `terraform plan` | **Automatic** — controller detects and fixes drift |
| Multi-cloud | Separate providers, same HCL | **Cloud-agnostic Compositions** — change a label to switch clouds |
| Integration | External CLI tool | **Native Kubernetes** — same `kubectl` workflow |
| GitOps | Possible but not native | **Native** with ArgoCD (Crossplane CRDs in Git → ArgoCD applies) |

### 8.3 Terraform → Crossplane Migration Map

| Current Terraform | Crossplane Replacement |
|-------------------|----------------------|
| `terraform/rds.tf` (AWS RDS PostgreSQL) | Crossplane `RDSInstance` CRD → wrapped in a `Composition` for cloud-agnostic `PostgreSQLInstance` |
| `terraform/ecs.tf` (ECS Fargate) | **Eliminated** — workloads run directly on Kubernetes (EKS/AKS/GKE) |
| `terraform/vpc.tf` (AWS VPC) | Crossplane `VPC` CRD → wrapped in cloud-agnostic `Network` Composition |
| `terraform/alb.tf` (AWS ALB) | Kubernetes `Ingress` + cloud-specific `LoadBalancer` service (handled by K8s cloud controller) |
| `terraform/ecr.tf` (AWS ECR) | Crossplane `ECRRepository` CRD, or use Harbor (cloud-agnostic) |
| `terraform/cloudwatch.tf` (Logging) | Kubernetes-native logging (Fluentd/Loki) + Prometheus/Grafana |
| `terraform/iam.tf` (AWS IAM) | Crossplane `IAMRole` CRD → or IRSA for EKS, Workload Identity for GKE |

### 8.4 Crossplane Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                    KUBERNETES CLUSTER                           │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  CROSSPLANE CONTROLLER                                    │ │
│  │                                                           │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │ │
│  │  │ AWS Provider │  │ Azure        │  │ GCP Provider │   │ │
│  │  │              │  │ Provider     │  │              │   │ │
│  │  │ RDS, S3,     │  │ Azure SQL,   │  │ Cloud SQL,   │   │ │
│  │  │ SQS, etc.    │  │ Blob, etc.   │  │ GCS, etc.    │   │ │
│  │  └──────────────┘  └──────────────┘  └──────────────┘   │ │
│  └──────────────────────────────────────────────────────────┘ │
│                              │                                 │
│  ┌──────────────────────────▼──────────────────────────────┐  │
│  │  COMPOSITIONS (cloud-agnostic templates)                 │  │
│  │                                                          │  │
│  │  PostgreSQLInstance:                                      │  │
│  │    provider: aws  → creates AWS RDS                      │  │
│  │    provider: azure → creates Azure Database for Postgres │  │
│  │    provider: gcp  → creates GCP Cloud SQL                │  │
│  └──────────────────────────────────────────────────────────┘  │
│                              │                                 │
│  ┌──────────────────────────▼──────────────────────────────┐  │
│  │  CLAIMS (concrete instances)                             │  │
│  │                                                          │  │
│  │  nexoraa-db:                                             │  │
│  │    compositionRef: PostgreSQLInstance                     │  │
│  │    parameters:                                           │  │
│  │      storageGB: 20                                       │  │
│  │      version: "15"                                       │  │
│  │    labels:                                               │  │
│  │      provider: aws  ← change to "azure" to switch cloud │  │
│  └──────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────┘
```

### 8.5 Example Crossplane Composition

```yaml
# crossplane/compositions/postgresql.yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: postgresql-aws
  labels:
    provider: aws
spec:
  compositeTypeRef:
    apiVersion: nexoraa.io/v1alpha1
    kind: PostgreSQLInstance
  resources:
    - name: rds-instance
      base:
        apiVersion: rds.aws.crossplane.io/v1alpha1
        kind: RDSInstance
        spec:
          forProvider:
            engine: postgres
            engineVersion: "15"
            instanceClass: db.t3.micro
            masterUsername: nexoraa
            allocatedStorage: 20
            publiclyAccessible: false
          providerConfigRef:
            name: aws-provider
      patches:
        - fromFieldPath: "spec.parameters.storageGB"
          toFieldPath: "spec.forProvider.allocatedStorage"
---
# crossplane/claims/nexoraa-db.yaml
apiVersion: nexoraa.io/v1alpha1
kind: PostgreSQLInstance
metadata:
  name: nexoraa-db
spec:
  parameters:
    storageGB: 20
    version: "15"
  compositionSelector:
    matchLabels:
      provider: aws  # Change to "azure" or "gcp" for other clouds
```

---

## 9. ArgoCD — GitOps Continuous Delivery

### 9.1 What Is ArgoCD?

ArgoCD is a **declarative GitOps continuous delivery tool** for Kubernetes. It watches a Git repository containing Kubernetes manifests and ensures the cluster state matches what's in Git — continuously.

### 9.2 ArgoCD vs GitHub Actions

**They are NOT competitors — they are complementary layers:**

| | GitHub Actions | ArgoCD |
|---|---|---|
| **Role** | CI (build, test, scan, push images) | CD (deploy to K8s, drift correction) |
| **Where it runs** | GitHub's cloud runners | Inside your Kubernetes cluster |
| **Deployment model** | Push-based (script runs `kubectl apply`) | Pull-based (controller watches Git, auto-applies) |
| **Drift detection** | None — fire-and-forget | **Continuous** — if someone `kubectl edit` in prod, ArgoCD reverts |
| **Rollback** | Re-run old pipeline (manual) | Revert Git commit → auto-syncs (seconds) |
| **Visibility** | GitHub Actions UI (per-run) | ArgoCD UI (live cluster state, sync status, health) |

### 9.3 How They Work Together

```
Developer → git push
    │
    ▼
GitHub Actions (CI):
    1. go test ./...                    ← Run tests
    2. go build -o nexoraa-api          ← Build binary
    3. docker build → push to ECR       ← Build & push image
    4. Update K8s manifests             ← Commit new image tag to deploy repo
    │        (image: nexoraa-api:v1.2.3)
    │
    ▼
ArgoCD (CD):
    1. Detects manifest change in Git
    2. Compares desired state (Git) vs actual state (K8s cluster)
    3. Applies diff to cluster
    4. Reports sync status (Synced / OutOfSync / Degraded)
    5. Continuous reconciliation — auto-heals drift
```

### 9.4 ArgoCD Application Definition

```yaml
# argocd/apps/nexoraa-api.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: nexoraa-api
  namespace: argocd
spec:
  project: nexoraa
  source:
    repoURL: https://github.com/narayana-reddy-nexoraa/platform
    targetRevision: main
    path: k8s/api                # Kubernetes manifests for API service
  destination:
    server: https://kubernetes.default.svc
    namespace: nexoraa-production
  syncPolicy:
    automated:
      prune: true                # Delete resources removed from Git
      selfHeal: true             # Auto-revert manual changes
    syncOptions:
      - CreateNamespace=true
```

---

## 10. Backstage — Internal Developer Portal

### 10.1 What Is Backstage?

Backstage (by Spotify) is an **Internal Developer Portal (IDP)**. It provides a single pane of glass where engineers can discover services, read documentation, scaffold new projects, and monitor health — all in one place.

### 10.2 Backstage vs GitHub Actions

**They are NOT related — completely different tools:**

| | GitHub Actions | Backstage |
|---|---|---|
| **Purpose** | Run automated CI/CD pipelines | Developer portal for service discovery, docs, templates |
| **Analogy** | The factory assembly line | The factory's control room and directory |
| **Who uses it** | CI/CD pipelines (automated) | Engineers (manual browsing, searching, scaffolding) |

### 10.3 Backstage Features for Nexoraa

| Feature | What It Does | Nexoraa Use Case |
|---------|-------------|-----------------|
| **Service Catalog** | Registry of all microservices, APIs, libraries with ownership, lifecycle status | Catalog all 25 SOPs, execution engine, integration connectors, studio services |
| **TechDocs** | Markdown docs rendered in-portal (docs-as-code) | SOP documentation, API specs, architecture decision records, runbooks |
| **Software Templates** | Scaffolding for new services (like `create-react-app` but for microservices) | "Create New SOP Service" → generates Go code + Dockerfile + K8s manifests + CI pipeline |
| **Plugins** | Extensible plugin ecosystem | ArgoCD plugin (deploy status), Grafana plugin (metrics), PagerDuty plugin (incidents), Kafka plugin (topic health) |
| **Search** | Unified search across all catalog entries, docs, APIs | "Who owns FS-01?" → shows team, owner, on-call, docs, health |

### 10.4 Example Backstage Catalog Entry

```yaml
# catalog-info.yaml (in platform repo root)
apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: nexoraa-execution-engine
  description: Go-based execution engine for SOP workflow orchestration
  annotations:
    github.com/project-slug: narayana-reddy-nexoraa/platform
    argocd/app-name: nexoraa-api
    grafana/dashboard-selector: nexoraa-api
    prometheus/rule-namespace: nexoraa
spec:
  type: service
  lifecycle: production
  owner: team-platform
  system: nexoraa-ai-platform
  dependsOn:
    - resource:nexoraa-db
    - resource:nexoraa-kafka
    - component:temporal-server
  providesApis:
    - nexoraa-execution-api
```

---

## 11. DevOps Stack — How It All Fits Together

### 11.1 The Complete Stack

```
┌─────────────────────────────────────────────────────────────────┐
│                    DEVELOPER WORKFLOW                             │
│                                                                  │
│  Developer writes code                                           │
│       │                                                          │
│       ▼                                                          │
│  GitHub Actions (CI)                                             │
│  ├── go test ./...                                               │
│  ├── go vet + golangci-lint                                      │
│  ├── docker build + push to ECR/ACR/GCR                          │
│  ├── Crossplane manifest update (if infra changed)               │
│  └── K8s manifest update (new image tag)                         │
│       │                                                          │
│       ▼                                                          │
│  ArgoCD (CD)                                                     │
│  ├── Detects Git change → applies to Kubernetes                  │
│  ├── Syncs Crossplane CRDs → provisions cloud resources          │
│  ├── Syncs app manifests → deploys services                      │
│  └── Continuous drift correction                                 │
│       │                                                          │
│       ▼                                                          │
│  Kubernetes (EKS / AKS / GKE)                                   │
│  ├── Nexoraa API Service (Go)                                    │
│  ├── Nexoraa Worker Service (Go + Temporal Worker)               │
│  ├── Temporal Server                                             │
│  ├── Kafka Cluster (Strimzi operator on K8s)                     │
│  ├── Nexoraa AI Studio (Dify-based)                              │
│  └── Crossplane Controller (manages cloud resources)             │
│       │                                                          │
│       ▼                                                          │
│  Crossplane (Multi-Cloud Infra)                                  │
│  ├── PostgreSQL (AWS RDS / Azure DB / GCP Cloud SQL)             │
│  ├── Object Storage (S3 / Blob / GCS)                            │
│  ├── Container Registry (ECR / ACR / GCR)                        │
│  └── Network (VPC / VNet / VPC)                                  │
│       │                                                          │
│       ▼                                                          │
│  Backstage (Developer Portal)                                    │
│  ├── Service Catalog (all 25 SOPs + infra + services)            │
│  ├── TechDocs (docs-as-code from repo)                           │
│  ├── Templates ("Create New SOP" scaffolding)                    │
│  └── Plugins (ArgoCD, Grafana, PagerDuty, Kafka)                │
└─────────────────────────────────────────────────────────────────┘
```

### 11.2 Tool Selection Summary

| Layer | Tool | Purpose | Replaces |
|-------|------|---------|----------|
| **CI** | GitHub Actions | Build, test, scan, push images | Manual builds, `scripts/deploy.sh` |
| **CD** | ArgoCD | GitOps deployment to K8s, drift correction | `kubectl apply`, ECS deploy |
| **Infra** | Crossplane | Multi-cloud resource provisioning | `terraform/` (AWS-only) |
| **Orchestration** | Temporal | Durable SOP workflow execution | Claimer/Publisher/Consumer/Reaper |
| **Streaming** | Kafka | High-volume event streaming, audit | In-process `chan` (1000 buffer) |
| **Compute** | Kubernetes (EKS/AKS/GKE) | Container orchestration | ECS Fargate (AWS-only) |
| **Portal** | Backstage | Service catalog, docs, templates | Tribal knowledge, scattered docs |
| **Studio** | Dify (Nexoraa AI Studio) | Visual workflow builder, SOP UI | Nothing (new) |
| **Monitoring** | Prometheus + Grafana | Metrics and dashboards | Already in place |
| **Logging** | OpenSearch | Log aggregation and search | CloudWatch (AWS-only) |
| **Tracing** | OpenTelemetry + Jaeger | Distributed tracing | Not present |

---

## 12. Updated Platform Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           NEXORAA PLATFORM v2                                 │
│                                                                               │
│  ┌────────────────────────────────────────────────────────────────────────┐   │
│  │                         CLIENT LAYER                                   │   │
│  │                                                                        │   │
│  │  ┌──────────────────┐  ┌───────────────────┐  ┌────────────────────┐  │   │
│  │  │  Nexoraa AI       │  │  Client Dashboard  │  │  IRA Voice        │  │   │
│  │  │  Studio (Dify)    │  │  (React)           │  │  (SandLogic)      │  │   │
│  │  │  Workflow Builder  │  │  SOP monitoring,   │  │  Voice agents,    │  │   │
│  │  │  SOP Templates     │  │  HITL workbench,   │  │  campaigns,       │  │   │
│  │  │  Knowledge Studio  │  │  Analytics         │  │  WebRTC test      │  │   │
│  │  └────────┬──────────┘  └────────┬───────────┘  └────────┬──────────┘  │   │
│  └───────────┼──────────────────────┼────────────────────────┼────────────┘   │
│              │                      │                        │                 │
│  ┌───────────▼──────────────────────▼────────────────────────▼────────────┐   │
│  │                         GO API SERVICE (Gin)                            │   │
│  │                                                                        │   │
│  │  /api/v1/executions    (existing execution CRUD)                       │   │
│  │  /api/v1/sops/:id/execute    (start Temporal workflow)                 │   │
│  │  /api/v1/sops/:wf_id/signal  (HITL approval signal)                   │   │
│  │  /api/v1/sops/:wf_id/status  (query workflow state)                   │   │
│  │  /api/v1/tenants       (tenant management)                            │   │
│  │                                                                        │   │
│  │  Middleware: Rate Limiting → Auth → Tenant → Backpressure → Tracing   │   │
│  └────────────────────────────────┬───────────────────────────────────────┘   │
│                                   │                                           │
│  ┌────────────────────────────────▼───────────────────────────────────────┐   │
│  │                      TEMPORAL (Workflow Orchestration)                  │   │
│  │                                                                        │   │
│  │  Workflows:                    Task Queues:                            │   │
│  │  ├── FNOLWorkflow              ├── "financial-services"                │   │
│  │  ├── KYCWorkflow               ├── "insurance"                        │   │
│  │  ├── AMLWorkflow               ├── "healthcare"                       │   │
│  │  ├── PriorAuthWorkflow         ├── "hospital-ops"                     │   │
│  │  ├── BedManagementWorkflow     ├── "life-sciences"                    │   │
│  │  ├── PharmacovigilanceWorkflow └── "manufacturing"                    │   │
│  │  ├── WorkOrderWorkflow                                                │   │
│  │  └── ... (25 total)            Signals: HITL approval/rejection       │   │
│  └────────────────────────────────┬───────────────────────────────────────┘   │
│                                   │                                           │
│  ┌────────────────────────────────▼───────────────────────────────────────┐   │
│  │                      KAFKA (Event Streaming)                           │   │
│  │                                                                        │   │
│  │  Topics:                        Features:                              │   │
│  │  ├── sop.executions.events      ├── Spike absorption (partitions)     │   │
│  │  ├── sop.hitl.requests          ├── Burst handling (auto-scale)       │   │
│  │  ├── integrations.ehr.adt       ├── Flipflop protection (cooldown)    │   │
│  │  ├── integrations.erp.events    ├── Backpressure (consumer lag)       │   │
│  │  ├── audit.trail                ├── Dead letter queues                │   │
│  │  └── dlq.*                      └── 7-day retention (audit replay)    │   │
│  └────────────────────────────────────────────────────────────────────────┘   │
│                                                                               │
│  ┌──────────────────────────────────────┐  ┌──────────────────────────────┐  │
│  │  CROSSPLANE (Multi-Cloud Infra)       │  │  ARGOCD (GitOps CD)          │  │
│  │                                       │  │                              │  │
│  │  Compositions:                        │  │  Apps:                       │  │
│  │  ├── PostgreSQLInstance               │  │  ├── nexoraa-api             │  │
│  │  ├── KafkaCluster                     │  │  ├── nexoraa-worker          │  │
│  │  ├── ObjectStorage                    │  │  ├── nexoraa-studio          │  │
│  │  └── Network                          │  │  ├── temporal                │  │
│  │                                       │  │  ├── kafka (Strimzi)         │  │
│  │  Providers: AWS / Azure / GCP         │  │  └── crossplane-resources    │  │
│  └──────────────────────────────────────┘  └──────────────────────────────┘  │
│                                                                               │
│  ┌──────────────────────────────────────────────────────────────────────────┐ │
│  │  BACKSTAGE (Developer Portal)                                            │ │
│  │  Service Catalog | TechDocs | Templates | ArgoCD + Grafana + Kafka UI    │ │
│  └──────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 13. Implementation Status (Roadmap — All Complete)

All 5 sprints are complete. Commits on `main`:

| Sprint | Weeks | Commit | Key Deliverables | Status |
|--------|-------|--------|-----------------|--------|
| **Sprint 1** | 1–2 | `d893acf` | SOP domain (25 defs), Temporal (SOPWorkflow + BridgeWorkflow + 7 activities), 3 DB migrations, `cmd/temporal-worker` | **Done** |
| **Sprint 2** | 3–4 | `43e8459` | Kafka broker (franz-go): producer, consumer, topics, adaptive consumer, backpressure, circuit breaker, lag monitor (12 files, 805 LOC) | **Done** |
| **Sprint 3** | 5–6 | `ce9a8ff` | API v2 (SOP/HITL/Audit handlers+services+repos), outbox→Kafka migration, React UI (Dashboard, HITL Workbench, Analytics) (27 files, 1,926 LOC) | **Done** |
| **Sprint 4** | 7–8 | `904d4ca` | Crossplane (4 compositions, 3 env claims), ArgoCD (app-of-apps), K8s manifests (base + 3 overlays), Backstage (25 SOP catalog + scaffolder) (45 files, 4,291 LOC) | **Done** |
| **Sprint 5** | 9–10 | `c274afa` | E2E tests (6), compliance validators (HIPAA, 21CFR11, BSA/AML, SOX), k6 load tests, prod K8s overlay (20 files, 2,240 LOC) | **Done** |
| **Post-sprint** | — | `682c3c5` | GitHub Actions CI/CD, EKS Terraform, bootstrap script, FewShotExamples, BridgeWorkflow, UserContextSignal | **Done** |

### Remaining Work (Not Yet Implemented)

| Area | Gap | Priority |
|------|-----|----------|
| **LLM Integration** | Activities return simulated results; need actual LLM API calls in Classification/Decisioning | **HIGH** |
| **Authentication** | No auth system — API uses raw `X-Tenant-ID` header | **HIGH** |
| **EKS Deployment** | Terraform + bootstrap script ready but not applied to AWS | **HIGH** |
| **Backstage Runtime** | Catalog files ready; Backstage app not containerized | **LOW** |
| **Azure/GCP Crossplane** | Only AWS provider configured | **MEDIUM** |

---

## Appendix A: Technology Decision Record

| # | Decision | Choice | Alternatives Considered | Rationale |
|---|----------|--------|------------------------|-----------|
| 1 | Workflow orchestration | **Temporal** | Cadence, custom state machine, Airflow | Durable execution, native HITL (Signals), Go SDK, active community |
| 2 | Message broker | **Kafka** | RabbitMQ, NATS, Pulsar | Event replay for audit, high throughput for real-time SOPs, natural outbox evolution |
| 3 | Multi-cloud IaC | **Crossplane** | Terraform, Pulumi, CDK | K8s-native, continuous reconciliation, ArgoCD integration, label-based cloud switching |
| 4 | GitOps CD | **ArgoCD** | FluxCD, Spinnaker, Jenkins X | Most mature K8s GitOps tool, excellent UI, Backstage plugin available |
| 5 | Developer portal | **Backstage** | Port, Cortex, custom wiki | Open-source (Spotify), huge plugin ecosystem, templates for SOP scaffolding |
| 6 | Container orchestration | **Kubernetes (EKS → multi)** | ECS Fargate (current) | Required for Crossplane, ArgoCD, Temporal operator, Kafka operator (Strimzi) |
| 7 | Studio / Workflow UI | **Dify (existing)** | Custom, Langflow, Flowise | Already deployed and in use by team; "Create from SOP" feature built |
| 8 | Voice channel | **IRA by SandLogic** | Custom, Twilio | Partnership in place; handles STT/LLM/TTS complexity |
| 9 | API Gateway | **Not needed — Gin middleware** | Kong, AWS API Gateway, Traefik | See Appendix B below |

---

## Appendix B: API Gateway Decision — Why We Don't Need One

### Decision: No External API Gateway

The platform is a **monorepo with a single Go API binary**. All routes (`/api/v1/*`, `/api/v2/*`) are served by one Gin HTTP server. An external API gateway would add an unnecessary network hop with no benefit.

### When API Gateways ARE Needed

| Scenario | Need Gateway? | Why |
|----------|:---:|-----|
| **Multiple repos/microservices** (separate teams, separate deploys) | **Yes** | Routes `/api/users` → User Service, `/api/orders` → Order Service. Single entry point for clients. |
| **Monorepo, single API binary** (our setup) | **No** | Everything is in one Go binary — Gin already handles routing, middleware, versioning. A gateway adds a useless hop. |
| **Monorepo, multiple binaries behind K8s** | **Maybe** | K8s Ingress acts as a lightweight gateway (TLS, routing) — not a full API gateway. |

### What Gin Middleware Provides Instead

The Gin middleware chain already covers the core API gateway responsibilities:

```
Client → Gin Middleware Chain:
           ├── TLS termination (handled by K8s Ingress when deployed to EKS)
           ├── Rate Limiter (existing token bucket in internal/broker/backpressure.go — needs wiring as Gin middleware)
           ├── Auth (future — JWT/OAuth, not yet implemented)
           ├── Tenant Extraction (implemented — middleware/tenant.go)
           ├── Correlation ID (implemented — middleware/correlation.go)
           ├── Error Handler (implemented — middleware/error.go)
           └── Request Logger (implemented — middleware/request_logger.go)
         → Handler → Service → Repository
```

### Current State vs. What's Still Needed

| Gateway Feature | Status | Location |
|----------------|--------|----------|
| Request routing (v1/v2) | **Done** | Gin route groups in `cmd/api/main.go` |
| Tenant extraction | **Done** | `internal/middleware/tenant.go` |
| Correlation ID / tracing | **Done** | `internal/middleware/correlation.go` |
| Request logging | **Done** | `internal/middleware/request_logger.go` |
| Error handling | **Done** | `internal/middleware/error.go` |
| Rate limiting (per-tenant) | **Code exists, not wired** | `internal/broker/backpressure.go` has `TenantRateLimiter` — needs to be wired as Gin middleware |
| Authentication (JWT/OAuth) | **Not implemented** | Needed before production — raw `X-Tenant-ID` header is insecure |
| TLS termination | **Not needed locally** | K8s Ingress handles TLS in production (EKS + cert-manager or ACM) |

### When to Reconsider

Revisit this decision if:
- The platform splits into **3+ separate services** with independent deploy cycles
- External partners need **API key management** with per-key rate limits and quotas
- You add a **public developer API** that needs OAuth2 client credentials flow

Until then, Gin middleware is the right approach — simpler, fewer moving parts, zero additional latency.
