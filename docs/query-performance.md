# Query Performance Analysis

This document provides a comprehensive analysis of the critical SQL queries in the narayana execution engine, their expected query plans, index coverage, and database maintenance guidance.

---

## 1. Critical Query Analysis

### 1.1 FindClaimableExecutions

**Frequency:** Every 2 seconds per worker (hot path)
**Latency impact:** Directly determines claim throughput; slowness here starves workers

```sql
SELECT execution_id, tenant_id, status, attempt_count, max_attempts,
       locked_by, lock_expires_at, version, retry_after, created_at, updated_at
FROM executions
WHERE status IN ('CREATED', 'FAILED')
  AND (locked_by IS NULL OR lock_expires_at < NOW())
  AND attempt_count < max_attempts
  AND (retry_after IS NULL OR retry_after <= NOW())
ORDER BY created_at ASC
LIMIT $1;
```

**Why it is critical:** This is the highest-frequency query in the system. Every worker instance polls this every 2 seconds. In a 10-worker deployment, that is 5 queries/second against the same rows. Any degradation (e.g., seq scan, lock contention) directly reduces claim throughput and increases execution latency.

**Key indexes used:**
- `idx_executions_claimable` -- partial index on `(tenant_id, status, lock_expires_at) WHERE status IN ('CREATED', 'FAILED')`
- `idx_executions_retry_after` -- partial index on `(retry_after) WHERE retry_after IS NOT NULL AND status IN ('CREATED', 'FAILED')`

**Expected EXPLAIN ANALYZE:**

```
Limit  (cost=0.29..12.45 rows=10 width=200) (actual time=0.032..0.058 rows=10 loops=1)
  ->  Index Scan using idx_executions_claimable on executions  (cost=0.29..156.80 rows=128 width=200) (actual time=0.031..0.054 rows=10 loops=1)
        Filter: ((attempt_count < max_attempts) AND ((locked_by IS NULL) OR (lock_expires_at < now())) AND ((retry_after IS NULL) OR (retry_after <= now())))
        Rows Removed by Filter: 2
Planning Time: 0.125 ms
Execution Time: 0.078 ms
```

**Notes:** The partial index `idx_executions_claimable` narrows the scan to only `CREATED`/`FAILED` rows. The `locked_by IS NULL OR lock_expires_at < NOW()` and `retry_after` filters are applied as in-index or post-index filters. The `LIMIT $1` (typically 10) ensures early termination. As the table grows, only the partial index size matters -- terminal-state rows (`SUCCEEDED`, `CANCELED`, etc.) are excluded.

---

### 1.2 FindExpiredLeases

**Frequency:** Every 10 seconds by the reaper goroutine
**Latency impact:** Delayed reaping means stuck executions; workers wait for the reaper to free leaked locks

```sql
SELECT execution_id, tenant_id, status, attempt_count, max_attempts,
       locked_by, lock_expires_at, version, retry_after, created_at, updated_at
FROM executions
WHERE locked_by IS NOT NULL
  AND lock_expires_at < NOW()
  AND status IN ('CLAIMED', 'RUNNING')
ORDER BY lock_expires_at ASC
LIMIT $1;
```

**Why it is critical:** The reaper is the system's self-healing mechanism. If a worker crashes without releasing its lease, this query finds the orphaned executions and reclaims them. While it runs less frequently (every 10s), slow execution here delays crash recovery, potentially causing SLA violations.

**Key indexes used:**
- `idx_executions_expired_leases` -- partial index on `(lock_expires_at) WHERE locked_by IS NOT NULL`

**Expected EXPLAIN ANALYZE:**

```
Limit  (cost=0.15..4.32 rows=10 width=200) (actual time=0.018..0.029 rows=3 loops=1)
  ->  Index Scan using idx_executions_expired_leases on executions  (cost=0.15..8.75 rows=20 width=200) (actual time=0.017..0.026 rows=3 loops=1)
        Index Cond: (lock_expires_at < now())
        Filter: (status = ANY ('{CLAIMED,RUNNING}'::execution_status[]))
        Rows Removed by Filter: 0
Planning Time: 0.098 ms
Execution Time: 0.041 ms
```

**Notes:** The partial index on `lock_expires_at WHERE locked_by IS NOT NULL` is highly selective -- only actively-locked rows are indexed. The `ORDER BY lock_expires_at ASC` matches the index order, avoiding a sort. Under normal operation, the result set is near-zero (only crashed workers leave expired leases).

---

### 1.3 ClaimExecution

**Frequency:** Runs on every claim attempt (up to `ClaimBatchSize` times per poll cycle, default 10)
**Latency impact:** Each claim is a serialized write; latency here directly bounds per-worker throughput

```sql
UPDATE executions
SET status = 'CLAIMED',
    locked_by = $2,
    lock_expires_at = NOW() + INTERVAL '1 second' * $3,
    attempt_count = attempt_count + 1,
    version = version + 1
WHERE execution_id = $1
  AND version = $4
  AND (status IN ('CREATED', 'FAILED'))
  AND (locked_by IS NULL OR lock_expires_at < NOW())
RETURNING *;
```

**Why it is critical:** This is a point UPDATE guarded by optimistic locking (`version = $4`). Under high concurrency, multiple workers race to claim the same execution. The loser gets 0 rows affected and retries the next candidate. The cost per attempt must stay below 1ms to maintain throughput.

**Key indexes used:**
- `executions_pkey` -- primary key index on `(execution_id)` for the `WHERE execution_id = $1` lookup

**Expected EXPLAIN ANALYZE:**

```
Update on executions  (cost=0.15..8.17 rows=1 width=200) (actual time=0.095..0.098 rows=1 loops=1)
  ->  Index Scan using executions_pkey on executions  (cost=0.15..8.17 rows=1 width=200) (actual time=0.022..0.024 rows=1 loops=1)
        Index Cond: (execution_id = $1)
        Filter: ((version = $4) AND (status = ANY ('{CREATED,FAILED}'::execution_status[])) AND ((locked_by IS NULL) OR (lock_expires_at < now())))
        Rows Removed by Filter: 0
Planning Time: 0.110 ms
Execution Time: 0.112 ms
```

**Notes:** This is a single-row update by primary key -- the most efficient UPDATE possible. The `version` check provides optimistic concurrency control. The `RETURNING *` avoids a follow-up SELECT. The `updated_at` trigger fires but adds negligible overhead.

---

### 1.4 CreateExecution

**Frequency:** Runs on every API `POST /api/v1/executions` request
**Latency impact:** Directly affects API response time (p99 target: < 10ms)

```sql
INSERT INTO executions (
    tenant_id, idempotency_key, status, attempt_count, max_attempts,
    payload, payload_hash, version
) VALUES (
    $1, $2, 'CREATED', 0, $3, $4, $5, 1
)
ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
RETURNING *;
```

**Why it is critical:** Every execution begins here. The `ON CONFLICT DO NOTHING` provides idempotency: a duplicate request returns no rows (the service then fetches the existing execution and compares payload hashes). INSERT latency under load determines API throughput.

**Key indexes used:**
- `uq_tenant_idempotency` -- unique constraint on `(tenant_id, idempotency_key)` for conflict detection

**Expected EXPLAIN ANALYZE:**

```
Insert on executions  (cost=0.00..0.01 rows=0 width=0) (actual time=0.145..0.148 rows=1 loops=1)
  Conflict Resolution: NOTHING
  Conflict Arbiter Indexes: uq_tenant_idempotency
  Tuples Inserted: 1
  Conflicting Tuples: 0
Planning Time: 0.085 ms
Execution Time: 0.162 ms
```

**Notes:** The unique index `uq_tenant_idempotency` serves double duty: it enforces the business rule (one execution per tenant+idempotency key) and powers the `ON CONFLICT` check. For idempotent retries, the conflict path is essentially free -- it checks the index, finds the conflict, and returns 0 rows with no write amplification.

---

### 1.5 GetExecutionByID

**Frequency:** Runs on every `GET /api/v1/executions/:id` request
**Latency impact:** Directly affects API read latency

```sql
SELECT * FROM executions
WHERE execution_id = $1 AND tenant_id = $2;
```

**Why it is critical:** Simple point lookup, but it is the most common read query. The `tenant_id` filter provides multi-tenant data isolation. This must always use an index scan.

**Key indexes used:**
- `executions_pkey` -- primary key index on `(execution_id)`

**Expected EXPLAIN ANALYZE:**

```
Index Scan using executions_pkey on executions  (cost=0.15..8.17 rows=1 width=200) (actual time=0.015..0.017 rows=1 loops=1)
  Index Cond: (execution_id = $1)
  Filter: (tenant_id = $2)
  Rows Removed by Filter: 0
Planning Time: 0.072 ms
Execution Time: 0.025 ms
```

**Notes:** The primary key lookup is O(log n). The `tenant_id` filter is applied as a post-index filter, which is fine for a single-row result. If cross-tenant access attempts occur (e.g., wrong tenant header), the filter removes the row and returns 0 results -- providing security-by-design at the query level.

---

## 2. Index Audit

### 2.1 All Indexes

| # | Index Name | Table | Columns | Condition (if partial) | Migration |
|---|-----------|-------|---------|----------------------|-----------|
| 1 | `executions_pkey` | executions | `(execution_id)` | -- | 000002 (implicit) |
| 2 | `uq_tenant_idempotency` | executions | `(tenant_id, idempotency_key)` | -- | 000002 |
| 3 | `idx_executions_claimable` | executions | `(tenant_id, status, lock_expires_at)` | `WHERE status IN ('CREATED', 'FAILED')` | 000003 |
| 4 | `idx_executions_expired_leases` | executions | `(lock_expires_at)` | `WHERE locked_by IS NOT NULL` | 000003 |
| 5 | `idx_executions_tenant_status` | executions | `(tenant_id, status, created_at DESC)` | -- | 000003 |
| 6 | `idx_executions_retry_after` | executions | `(retry_after)` | `WHERE retry_after IS NOT NULL AND status IN ('CREATED', 'FAILED')` | 000010 |
| 7 | `idx_transitions_execution` | execution_transitions | `(execution_id, created_at)` | -- | 000005 |
| 8 | `idx_outbox_unsent` | outbox_events | `(sequence_number ASC)` | `WHERE sent = FALSE` | 000006 |
| 9 | `idx_outbox_cleanup` | outbox_events | `(sent_at)` | `WHERE sent = TRUE` | 000006 |
| 10 | `idx_outbox_aggregate` | outbox_events | `(aggregate_type, aggregate_id, sequence_number)` | -- | 000006 |
| 11 | `idx_processing_log_execution` | processing_log | `(execution_id, created_at)` | -- | 000006 |
| 12 | `idx_dlq_consumer_group` | dead_letter_events | `(consumer_group, created_at DESC)` | -- | 000007 |
| 13 | `idx_dlq_event_id` | dead_letter_events | `(event_id, consumer_group)` | -- (UNIQUE) | 000009 |

### 2.2 Query-to-Index Mapping

| Query | Primary Index Used | Optimal? |
|-------|-------------------|----------|
| `FindClaimableExecutions` | `idx_executions_claimable` | Yes -- partial index excludes terminal states |
| `FindExpiredLeases` | `idx_executions_expired_leases` | Yes -- partial index on `locked_by IS NOT NULL` |
| `ClaimExecution` | `executions_pkey` | Yes -- point update by PK |
| `CreateExecution` | `uq_tenant_idempotency` | Yes -- powers ON CONFLICT |
| `GetExecutionByID` | `executions_pkey` | Yes -- point lookup by PK |
| `ListExecutions` | `idx_executions_tenant_status` | Yes -- covers `(tenant_id, status, created_at DESC)` |
| `CountExecutions` | `idx_executions_tenant_status` | Partial -- index-only scan possible but COUNT still reads all matching rows |
| `SendHeartbeat` | `executions_pkey` | Yes -- point update by PK |
| `CompleteExecution` | `executions_pkey` | Yes -- point update by PK |
| `FailExecution` | `executions_pkey` | Yes -- point update by PK |
| `RetryExecution` | `executions_pkey` | Yes -- point update by PK |
| `ReclaimExecution` | `executions_pkey` | Yes -- point update by PK |

### 2.3 Missing Index Opportunities

**1. `FindClaimableExecutions` ordering by `created_at ASC`**

The current `idx_executions_claimable` index is on `(tenant_id, status, lock_expires_at)`, but `FindClaimableExecutions` does not filter by `tenant_id` and orders by `created_at ASC`. A more targeted index could be:

```sql
CREATE INDEX idx_executions_claimable_v2
    ON executions (created_at ASC)
    WHERE status IN ('CREATED', 'FAILED');
```

This would allow the planner to use the index ordering directly, avoiding a sort. However, the current `LIMIT 10` makes sorting cheap (top-N heapsort), so this is a micro-optimization. **Verdict: not urgent, monitor with `pg_stat_statements` first.**

**2. `CountActiveExecutions` and `CountPendingExecutions`**

These aggregate queries scan all matching rows:

```sql
-- CountActiveExecutions
SELECT COUNT(*) FROM executions WHERE status IN ('CLAIMED', 'RUNNING');

-- CountPendingExecutions
SELECT COUNT(*) FROM executions WHERE status = 'CREATED';
```

The `idx_executions_tenant_status` index can provide an index-only scan if the visibility map is up to date (requires recent VACUUM). If these become slow at scale, consider partial indexes:

```sql
CREATE INDEX idx_executions_active ON executions (status) WHERE status IN ('CLAIMED', 'RUNNING');
CREATE INDEX idx_executions_pending ON executions (status) WHERE status = 'CREATED';
```

**Verdict: only add if `pg_stat_statements` shows mean_exec_time > 5ms for these queries. They are used for metrics gauges, not hot path.**

---

## 3. pg_stat_statements Setup

`pg_stat_statements` is the single most important tool for production query performance monitoring. It tracks execution statistics for every distinct query.

### 3.1 Enable the Extension

Add to `postgresql.conf` (or via Docker environment):

```
shared_preload_libraries = 'pg_stat_statements'
pg_stat_statements.track = all
```

Then in the database:

```sql
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

### 3.2 Top Slow Queries by Mean Execution Time

```sql
SELECT
    queryid,
    LEFT(query, 80) AS query_preview,
    calls,
    ROUND(mean_exec_time::numeric, 3) AS mean_ms,
    ROUND(total_exec_time::numeric, 1) AS total_ms,
    ROUND((stddev_exec_time)::numeric, 3) AS stddev_ms,
    rows
FROM pg_stat_statements
WHERE dbname = current_database()
ORDER BY mean_exec_time DESC
LIMIT 20;
```

### 3.3 Highest Total Time (cumulative impact)

```sql
SELECT
    queryid,
    LEFT(query, 80) AS query_preview,
    calls,
    ROUND(total_exec_time::numeric, 1) AS total_ms,
    ROUND(mean_exec_time::numeric, 3) AS mean_ms,
    ROUND((total_exec_time / SUM(total_exec_time) OVER ()) * 100, 2) AS pct_total
FROM pg_stat_statements
WHERE dbname = current_database()
ORDER BY total_exec_time DESC
LIMIT 20;
```

### 3.4 Monitor the Critical Queries

Use these `queryid`-based alerts or dashboards to track the five critical queries identified in Section 1. Expected baselines:

| Query | Expected mean_exec_time | Alert threshold |
|-------|------------------------|-----------------|
| `FindClaimableExecutions` | < 1ms | > 5ms |
| `FindExpiredLeases` | < 0.5ms | > 3ms |
| `ClaimExecution` | < 0.5ms | > 2ms |
| `CreateExecution` | < 0.5ms | > 3ms |
| `GetExecutionByID` | < 0.1ms | > 1ms |

### 3.5 Reset Statistics

After a deployment or configuration change, reset to get clean measurements:

```sql
SELECT pg_stat_statements_reset();
```

---

## 4. Database Maintenance

### 4.1 Autovacuum Verification

PostgreSQL's autovacuum is critical for this workload because:
- The `executions` table has high UPDATE churn (status transitions, lease claims, heartbeats)
- Dead tuples from updates must be cleaned to maintain index efficiency
- The visibility map must be current for index-only scans on COUNT queries

**Check autovacuum health:**

```sql
SELECT
    relname,
    n_live_tup,
    n_dead_tup,
    ROUND(n_dead_tup::numeric / GREATEST(n_live_tup, 1) * 100, 2) AS dead_pct,
    last_vacuum,
    last_autovacuum,
    last_analyze,
    last_autoanalyze,
    autovacuum_count,
    autoanalyze_count
FROM pg_stat_user_tables
WHERE schemaname = 'public'
ORDER BY n_dead_tup DESC;
```

**Expected output for a healthy system:**

| relname | n_dead_tup | dead_pct | last_autovacuum |
|---------|-----------|----------|-----------------|
| executions | < 5000 | < 10% | within last 5 min |
| outbox_events | < 2000 | < 5% | within last 10 min |
| execution_transitions | < 1000 | < 5% | within last 30 min |

**Warning signs:**
- `dead_pct` > 20% -- autovacuum is falling behind
- `last_autovacuum` is NULL or > 1 hour ago -- autovacuum may not be running
- `n_dead_tup` is growing monotonically -- check `autovacuum_max_workers` and table-level settings

### 4.2 Autovacuum Tuning for High-Churn Tables

For the `executions` table, consider more aggressive autovacuum settings:

```sql
ALTER TABLE executions SET (
    autovacuum_vacuum_scale_factor = 0.05,    -- trigger at 5% dead tuples (default 20%)
    autovacuum_analyze_scale_factor = 0.02,   -- re-analyze at 2% changes (default 10%)
    autovacuum_vacuum_cost_delay = 2          -- reduce throttling (default 20ms)
);
```

### 4.3 Table Bloat Check

Table bloat occurs when dead tuples accumulate faster than autovacuum can clean them. Use the `pgstattuple` extension:

```sql
CREATE EXTENSION IF NOT EXISTS pgstattuple;

SELECT
    table_len,
    tuple_count,
    tuple_len,
    dead_tuple_count,
    dead_tuple_len,
    ROUND(dead_tuple_len::numeric / GREATEST(table_len, 1) * 100, 2) AS bloat_pct,
    free_space,
    ROUND(free_space::numeric / GREATEST(table_len, 1) * 100, 2) AS free_pct
FROM pgstattuple('executions');
```

**Interpreting results:**
- `bloat_pct` < 10% -- healthy
- `bloat_pct` 10-30% -- monitor, autovacuum should catch up
- `bloat_pct` > 30% -- consider `VACUUM FULL` during maintenance window (takes exclusive lock)

**Alternative without `pgstattuple` (estimate from `pg_class`):**

```sql
SELECT
    relname,
    pg_size_pretty(pg_total_relation_size(c.oid)) AS total_size,
    pg_size_pretty(pg_relation_size(c.oid)) AS table_size,
    pg_size_pretty(pg_indexes_size(c.oid)) AS index_size,
    reltuples::bigint AS estimated_rows
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
  AND c.relkind = 'r'
ORDER BY pg_total_relation_size(c.oid) DESC;
```

### 4.4 Index Bloat Check

Indexes on frequently-updated columns (`status`, `lock_expires_at`, `version`) can bloat faster than the table:

```sql
SELECT
    indexrelname,
    pg_size_pretty(pg_relation_size(indexrelid)) AS index_size,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE schemaname = 'public'
ORDER BY pg_relation_size(indexrelid) DESC;
```

If an index is significantly larger than expected for the row count, consider `REINDEX CONCURRENTLY`:

```sql
REINDEX INDEX CONCURRENTLY idx_executions_claimable;
```

---

## 5. Connection Pool Configuration

### 5.1 Current Settings

Configured in `internal/config/config.go` and applied in both `cmd/api/main.go` and `cmd/worker/main.go`:

| Setting | Value | Env Override | Default pgxpool |
|---------|-------|-------------|-----------------|
| `MaxConns` | 20 | `DB_MAX_CONNS` | 4 |
| `MinConns` | 5 | `DB_MIN_CONNS` | 0 |
| `MaxConnLifetime` | 30m | `DB_MAX_CONN_LIFETIME` | 1h |
| `MaxConnIdleTime` | 5m | `DB_MAX_CONN_IDLE_TIME` | 30m |

### 5.2 Rationale

**MaxConns = 20 (default pgxpool is 4)**

The API server and worker both need concurrent database access:

- **API server:** Each HTTP request needs a connection for the duration of the query. At 100 concurrent requests, the pool must have enough connections to avoid queuing.
- **Worker:** The claimer, reaper, publisher, consumer, and gauge collector all run concurrently. The claimer alone may issue up to `ClaimBatchSize` (10) sequential updates per cycle.
- 20 connections per process provides headroom for concurrent API requests + background goroutines without exhausting PostgreSQL's default `max_connections` (100).

**Rule of thumb:** For a single-instance deployment, `MaxConns = 20` is appropriate. For multi-instance deployments, ensure `sum(MaxConns across all instances) < max_connections - 5` (reserve connections for superuser/monitoring).

**MinConns = 5**

Keeps 5 warm connections to avoid cold-start latency. The connection establishment cost (~5-10ms with TLS) is eliminated for the first 5 concurrent operations after idle periods.

**MaxConnLifetime = 30m**

Forces connection recycling every 30 minutes to:
- Pick up DNS changes (important for cloud-managed databases like RDS)
- Rebalance connections across read replicas behind a load balancer
- Prevent stale server-side session state from accumulating

**MaxConnIdleTime = 5m**

Closes idle connections after 5 minutes. This releases server-side resources during low-traffic periods while keeping the pool responsive during normal load. The `MinConns` setting ensures at least 5 connections survive idle periods.

### 5.3 Connection Budget Planning

For a multi-instance production deployment:

```
PostgreSQL max_connections = 100 (default)

API instances:     2 x 20 = 40 connections
Worker instances:  2 x 20 = 40 connections
Monitoring/admin:          =  5 connections
Superuser reserve:         =  5 connections
                            ─────────────
Total:                       90 connections (10 headroom)
```

If scaling beyond 2+2 instances, either:
1. Increase `max_connections` (up to ~200-300 is safe on modern hardware)
2. Deploy PgBouncer as a connection pooler in front of PostgreSQL
3. Reduce `MaxConns` per instance (e.g., 15 per instance x 3 = 45)
