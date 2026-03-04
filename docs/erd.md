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

    OUTBOX_EVENTS {
        UUID event_id PK "DEFAULT gen_random_uuid()"
        VARCHAR(100) aggregate_type "NOT NULL"
        UUID aggregate_id "NOT NULL"
        VARCHAR(100) event_type "NOT NULL"
        JSONB payload "NOT NULL"
        JSONB metadata "nullable"
        BIGSERIAL sequence_number "NOT NULL"
        TIMESTAMPTZ created_at "NOT NULL DEFAULT NOW()"
        TIMESTAMPTZ sent_at "nullable"
        BOOLEAN sent "NOT NULL DEFAULT FALSE"
    }

    PROCESSED_EVENTS {
        UUID event_id PK "NOT NULL"
        VARCHAR(100) consumer_group PK "NOT NULL"
        TIMESTAMPTZ processed_at "NOT NULL DEFAULT NOW()"
    }

    PROCESSING_LOG {
        UUID log_id PK "DEFAULT gen_random_uuid()"
        UUID execution_id FK "NOT NULL → executions"
        VARCHAR(255) worker_id "NOT NULL"
        VARCHAR(50) action "NOT NULL"
        INTEGER attempt_number "NOT NULL"
        TIMESTAMPTZ created_at "NOT NULL DEFAULT NOW()"
    }

    EXECUTIONS ||--o{ EXECUTION_TRANSITIONS : "has audit trail"
    EXECUTIONS ||--o{ PROCESSING_LOG : "has processing audit"
    OUTBOX_EVENTS ||--o| PROCESSED_EVENTS : "deduplication"
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

## Outbox & Processing Tables

### outbox_events Indexes

| Name | Columns | Condition | Purpose |
|------|---------|-----------|---------|
| `idx_outbox_unsent` | `(sequence_number ASC)` | `WHERE sent = FALSE` | Publisher polling |
| `idx_outbox_cleanup` | `(sent_at)` | `WHERE sent = TRUE` | Old event cleanup |
| `idx_outbox_aggregate` | `(aggregate_type, aggregate_id, sequence_number)` | — | Aggregate event history |

### processed_events

Composite primary key `(event_id, consumer_group)` — ensures each event is processed exactly once per consumer group.

### processing_log Indexes

| Name | Columns | Condition | Purpose |
|------|---------|-----------|---------|
| `idx_processing_log_execution` | `(execution_id, created_at)` | — | Execution processing audit |

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

---

## Sequence Diagrams

### Claim and Process Flow

```mermaid
sequenceDiagram
    participant W as Worker (Claimer)
    participant S as Service
    participant R as Repository
    participant DB as PostgreSQL
    participant H as Heartbeat

    W->>S: ClaimNextExecution(workerID)
    S->>R: FindClaimable(batchSize=5)
    R->>DB: SELECT ... WHERE status IN ('CREATED','FAILED')<br/>AND (locked_by IS NULL OR lock_expires_at < NOW())
    DB-->>R: candidates[]

    loop Try each candidate
        S->>R: ClaimWithOutbox(execID, workerID, leaseDuration, version)
        R->>DB: BEGIN TX
        Note over DB: UPDATE status=CLAIMED, locked_by=worker WHERE version=N
        Note over DB: INSERT execution_transitions
        Note over DB: INSERT outbox_events
        R->>DB: COMMIT
    end

    R-->>S: claimed Execution
    S-->>W: claimed Execution

    W->>S: UpdateStatus(execID, RUNNING, version)
    S->>R: UpdateStatus(...)
    R->>DB: UPDATE status=RUNNING WHERE version=N

    W->>H: Start heartbeat loop

    par Heartbeat Loop
        loop Every 10s
            H->>R: SendHeartbeat(execID, workerID)
            R->>DB: UPDATE lock_expires_at, last_heartbeat_at
        end
    and Processing
        Note over W: Execute business logic
    end

    W->>H: Stop heartbeat

    alt Success
        W->>S: CompleteExecution(execID, workerID, version)
        S->>R: CompleteWithOutbox(...)
        R->>DB: BEGIN TX
        Note over DB: UPDATE status=SUCCEEDED, locked_by=NULL
        Note over DB: INSERT execution_transitions
        Note over DB: INSERT outbox_events
        R->>DB: COMMIT
    else Retryable Failure
        W->>S: RetryExecution(execID, workerID, ...)
        S->>R: RetryWithOutbox(...)
        R->>DB: BEGIN TX (status=CREATED + transition + outbox)
    else Permanent Failure
        W->>S: FailExecution(execID, workerID, ...)
        S->>R: FailWithOutbox(...)
        R->>DB: BEGIN TX (status=FAILED + transition + outbox)
    end
```

### Event Publishing Flow

```mermaid
sequenceDiagram
    participant TX as Transactional Write
    participant DB as PostgreSQL
    participant P as Publisher
    participant CH as Go Channel (buf 1000)
    participant C as Consumer
    participant PE as processed_events
    participant DLQ as dead_letter_events
    participant H as Handler

    TX->>DB: INSERT INTO outbox_events (sent=FALSE)
    Note over DB: Atomic with state change + transition

    loop Every 2 seconds
        P->>DB: SELECT FROM outbox_events<br/>WHERE sent=FALSE ORDER BY sequence_number<br/>LIMIT 50 FOR UPDATE SKIP LOCKED
        DB-->>P: unsent events[]

        loop Each event
            P->>CH: send(event)
        end

        P->>DB: UPDATE SET sent=TRUE, sent_at=NOW()<br/>WHERE event_id IN (...)
    end

    loop Read from channel
        CH-->>C: receive event

        C->>PE: INSERT (event_id, consumer_group)<br/>ON CONFLICT DO NOTHING RETURNING event_id

        alt New event (row returned)
            PE-->>C: event_id (is_new=true)
            C->>H: handler(ctx, event)
            alt Handler succeeds
                H-->>C: nil
                Note over C: Log success, update offset
            else Handler fails
                H-->>C: error
                C->>DLQ: INSERT dead_letter_events
                Note over C: Log failure, update offset
            end
        else Duplicate (no row returned)
            PE-->>C: ErrNoRows (is_new=false)
            Note over C: Skip duplicate
        end
    end
```
