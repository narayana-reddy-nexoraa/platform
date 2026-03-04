# Entity Relationship Diagram

## Executions Table

```mermaid
erDiagram
    EXECUTIONS {
        UUID execution_id PK "DEFAULT gen_random_uuid()"
        UUID tenant_id "NOT NULL"
        VARCHAR(255) idempotency_key "NOT NULL"
        ENUM status "NOT NULL DEFAULT CREATED"
        INTEGER attempt_count "NOT NULL DEFAULT 0"
        INTEGER max_attempts "NOT NULL DEFAULT 3"
        VARCHAR(255) locked_by "nullable — worker ID"
        TIMESTAMPTZ lock_expires_at "nullable — lease expiry"
        TIMESTAMPTZ last_heartbeat_at "nullable"
        VARCHAR(100) error_code "nullable"
        TEXT error_message "nullable"
        JSONB payload "NOT NULL DEFAULT {}"
        VARCHAR(64) payload_hash "NOT NULL — SHA-256"
        TIMESTAMPTZ retry_after "nullable — earliest retry time"
        TIMESTAMPTZ created_at "NOT NULL DEFAULT NOW()"
        TIMESTAMPTZ updated_at "NOT NULL DEFAULT NOW()"
        INTEGER version "NOT NULL DEFAULT 1 — optimistic lock"
    }

    EXECUTION_TRANSITIONS {
        UUID transition_id PK "DEFAULT gen_random_uuid()"
        UUID execution_id FK "NOT NULL → executions"
        ENUM from_status "NOT NULL"
        ENUM to_status "NOT NULL"
        VARCHAR(255) triggered_by "NOT NULL — worker/reaper/system"
        TEXT reason "nullable"
        JSONB metadata "nullable"
        TIMESTAMPTZ created_at "NOT NULL DEFAULT NOW()"
    }

    EXECUTIONS ||--o{ EXECUTION_TRANSITIONS : "has audit trail"
```

### Constraints

| Name | Type | Definition |
|------|------|------------|
| `execution_id` | PRIMARY KEY | Auto-generated UUID |
| `uq_tenant_idempotency` | UNIQUE | `(tenant_id, idempotency_key)` |
| `chk_attempt_count` | CHECK | `attempt_count >= 0` |
| `chk_max_attempts` | CHECK | `max_attempts > 0` |
| `chk_version` | CHECK | `version > 0` |

### Indexes

| Name | Columns | Condition | Purpose |
|------|---------|-----------|---------|
| `idx_executions_claimable` | `(tenant_id, status, lock_expires_at)` | `WHERE status IN ('CREATED','FAILED')` | Worker claim queries |
| `idx_executions_expired_leases` | `(lock_expires_at)` | `WHERE locked_by IS NOT NULL` | Dead worker lease reaper |
| `idx_executions_tenant_status` | `(tenant_id, status, created_at DESC)` | — | API list/filter queries |
| `idx_transitions_execution` | `(execution_id, created_at)` | — | Audit trail lookups |

### Trigger

`trg_executions_updated_at` — auto-updates `updated_at` column on every row modification.

---

## Execution Status State Machine

```mermaid
stateDiagram-v2
    [*] --> CREATED

    CREATED --> CLAIMED : Worker claims
    CREATED --> CANCELED : User cancels

    CLAIMED --> RUNNING : Worker starts processing
    CLAIMED --> CREATED : Reaper reclaims expired lease
    CLAIMED --> CANCELED : User cancels
    CLAIMED --> TIMED_OUT : Lease expires

    RUNNING --> SUCCEEDED : Completed successfully
    RUNNING --> FAILED : Error during processing
    RUNNING --> CANCELED : User cancels
    RUNNING --> TIMED_OUT : Heartbeat missed

    FAILED --> CREATED : Retry (retryable error + attempts remain)
    FAILED --> CLAIMED : Legacy retry path
    FAILED --> CANCELED : User cancels

    TIMED_OUT --> CREATED : Retry after timeout

    SUCCEEDED --> [*]
    CANCELED --> [*]
```

### Terminal States
- **SUCCEEDED** — execution completed successfully
- **CANCELED** — execution canceled by user

### Non-Terminal Failure States
- **FAILED** — can be retried (→ CREATED) if error is retryable and attempts remain
- **TIMED_OUT** — can be retried (→ CREATED) via reaper recovery
