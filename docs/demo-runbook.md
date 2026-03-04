# Demo Runbook: Narayana Distributed Execution Engine

A step-by-step narration guide for the live IDP deep dive demo. Total estimated time: **~20 minutes** plus Q&A.

---

## Prerequisites

Before starting the demo, ensure:

- Docker Desktop is running with sufficient resources (4+ CPU cores, 4GB+ RAM)
- Ports 3001, 5432, 8080, 9091 are free
- The repository is cloned and you are in the project root
- Images are built (`docker compose build` has been run at least once)

---

## Step 1: Setup & Architecture Overview (~2 min)

### What to run

```bash
make demo-setup
```

This builds the Docker images and starts the full stack: PostgreSQL, migrations, 1 API server, 4 workers, Prometheus, and Grafana.

### What to show

1. **Terminal output** -- Confirm all containers are running:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.demo.yml ps
   ```
   You should see: `postgres`, `api`, `worker-1` through `worker-4`, `prometheus`, `grafana` all with status `Up`.

2. **Grafana** -- Open [http://localhost:3001](http://localhost:3001) (login: `admin` / `admin`).
   Navigate through the three provisioned dashboards:
   - **Overview** -- Execution counts by status, worker activity, claim/succeed/fail rates
   - **Performance** -- Request latency percentiles, database query times
   - **Event Pipeline** -- Outbox publish rate, consumer process rate, DLQ depth

### What to say

> "This is a distributed execution engine. One API server handles all HTTP requests -- creating executions with idempotency keys, querying status. Four workers independently claim and process executions using lease-based concurrency control. Everything is backed by PostgreSQL. There is no Redis, no Kafka, no external message broker. The outbox pattern gives us guaranteed event delivery within the same database transaction. Prometheus scrapes all services, and Grafana gives us full observability."

### What to prove

- All 8 containers are running and healthy
- Grafana loads with three dashboards showing live (but empty) metrics

---

## Step 2: Scenario 1 -- High Concurrency Safety (~3 min)

### What to run

```bash
make demo-scenario1
```

This creates 500 executions concurrently using 50 parallel clients, then waits for all of them to reach SUCCEEDED status.

### What to show

- **Grafana Overview dashboard** -- Watch execution rates climbing in real time. The `executions_claimed_total` and `executions_succeeded_total` counters should rise steadily.
- **Terminal output** -- Progress counter showing how many have reached SUCCEEDED. Final verification showing 500/500 SUCCEEDED, 0 duplicates.

### What to say

> "We are sending 500 execution requests concurrently from 50 parallel clients. Each request carries a unique idempotency key -- if the same key is sent twice, the system returns the existing execution rather than creating a duplicate. Workers claim executions using SELECT FOR UPDATE SKIP LOCKED, which means no two workers ever claim the same execution. Optimistic locking with a version column ensures exactly-once processing. Let's watch the dashboard -- you can see all four workers sharing the load evenly."

### What to prove

- **500/500 SUCCEEDED** -- Every execution completed
- **0 duplicate completions** -- The processing_log table confirms no execution was completed more than once
- No FAILED, RUNNING, or CLAIMED executions remain

---

## Step 3: Scenario 2 -- Crash Recovery (~4 min)

### What to run

```bash
make demo-scenario2
```

This creates 200 executions, waits for workers to start processing, then kills workers 3 and 4. After a 35-second wait for leases to expire, it verifies that workers 1 and 2 reclaim and complete all work.

### What to show

- **Grafana Overview dashboard** -- Watch for:
  - Active worker count dropping from 4 to 2
  - The `leases_reclaimed_total` metric spiking after ~30 seconds
  - Execution rates dipping then recovering
- **Terminal output** -- The script prints state before crash, after crash, and final verification.

### What to say

> "We have 200 executions in progress across 4 workers. Now I am killing 2 of the 4 workers -- simulating a crash, a node failure, a deployment gone wrong. Watch the Grafana dashboard. The active worker count drops from 4 to 2. The executions those workers had claimed are now orphaned -- they have a lease with an expiry time. After 30 seconds, those leases expire. The reaper process detects orphaned executions and reclaims them, resetting their status back to CREATED. The remaining 2 workers pick up the work and complete everything. No human intervention required."

### What to prove

- **All 200 SUCCEEDED** -- Despite losing half the workers mid-flight
- **Reclaim transitions visible** -- The `execution_transitions` table shows RUNNING -> CREATED transitions, proving the reaper reclaimed orphaned work
- Workers 3 and 4 are restarted at the end to restore full capacity

---

## Step 4: Scenario 3 -- Retry Storm (~4 min)

### What to run

```bash
make demo-scenario3
```

This restarts all workers with a 50% failure rate, creates 100 executions, watches the retry behavior for 30 seconds, then disables failures and waits for everything to succeed.

### What to show

- **Grafana Overview dashboard** -- Watch:
  - The `executions_retried_total` counter climbing steadily
  - The ratio of SUCCEEDED vs FAILED fluctuating
  - After failures are disabled, the remaining executions draining to SUCCEEDED
- **Terminal output** -- Progress updates every 5 seconds showing the current breakdown of SUCCEEDED, FAILED, and CREATED.

### What to say

> "Workers are now configured with a 50% failure rate -- every other execution processing attempt will fail. Watch the retry counter climb on the dashboard. When a worker fails to process an execution, the status goes back to CREATED with an incremented attempt count. Exponential backoff prevents a retry storm from overwhelming the database. The system handles this gracefully -- no cascading failures, no circuit breaker needed, just steady progress. Now I am disabling the failures... watch how all remaining executions drain to SUCCEEDED."

### What to prove

- **100/100 SUCCEEDED** -- Every execution eventually completed
- **Total attempts > 100** -- The sum of `attempt_count` across all executions proves retries occurred (e.g., 150+ total attempts for 100 executions)

---

## Step 5: Scenario 4 -- Event Pipeline (~3 min)

### What to run

```bash
make demo-scenario4
```

This creates 50 executions, waits for completion, then inspects the full event pipeline: outbox events, processed events, dead letter queue, and traces a single execution end-to-end.

### What to show

- **Grafana Event Pipeline dashboard** -- Outbox publish rate and consumer process rate should show matching throughput.
- **Terminal output** -- Detailed breakdown of:
  - Outbox events (total, sent, unsent)
  - Processed events (deduplicated consumer count)
  - Dead letter queue depth
  - Event types and their counts
  - Full trace of a single execution: transitions and corresponding events

### What to say

> "Every state change in the system produces an outbox event -- and it does so atomically, within the same database transaction as the state change itself. This is the outbox pattern: instead of publishing to an external broker, we write the event to an outbox table, then a publisher process polls for unsent events and delivers them to consumers. Consumers deduplicate using the processed_events table, so even if the same event is delivered twice, it is only processed once. Let me trace a single execution through the pipeline -- here you can see CREATED, then CLAIMED, then RUNNING, then SUCCEEDED, each with a corresponding outbox event. All events sent, all processed, zero in the dead letter queue."

### What to prove

- **All outbox events sent** -- 0 unsent events remaining
- **Processed count matches** -- Consumer processed all events
- **DLQ empty** -- 0 dead letter events
- **Single execution trace** -- Clear CREATED -> CLAIMED -> RUNNING -> SUCCEEDED progression with matching events

---

## Step 6: Full Verification (~1 min)

### What to run

```bash
make demo-verify
```

This runs a comprehensive verification across all data: execution status summary, stuck execution check, duplicate processing check, invalid transition check, outbox health, and DLQ status.

### What to show

- **Terminal output** -- The full verification report. All checks should pass.

### What to say

> "Let's run the full verification suite. This checks every invariant the system guarantees: no stuck executions, no duplicate processing, no invalid state transitions, all events delivered, empty dead letter queue."

### What to prove

- **0 stuck executions** -- No non-terminal executions older than 5 minutes
- **0 duplicate completions** -- No execution was completed more than once
- **0 invalid transitions** -- Every state transition follows the valid state machine
- **Outbox healthy** -- All events sent
- **DLQ empty** -- No poisoned events

---

## Step 7: Architecture Recap (~2 min)

### What to run

No commands -- narration only.

### What to say

> "Let me recap the key patterns that make this system reliable:
>
> 1. **Idempotent creation** -- Every execution has a tenant-scoped idempotency key. INSERT ON CONFLICT DO NOTHING plus payload hash comparison prevents duplicates at the database level.
>
> 2. **Lease-based claiming** -- Workers claim executions with SELECT FOR UPDATE SKIP LOCKED. Each claim sets a locked_by field and a lock_expires_at timestamp. No two workers can claim the same execution.
>
> 3. **Optimistic locking** -- Every mutation checks WHERE version = $expected and increments the version. If two processes race, one wins and the other gets a conflict error and retries.
>
> 4. **Heartbeat and reaper for crash recovery** -- Workers send heartbeats to extend their leases. If a worker crashes, the lease expires. The reaper detects orphaned executions and returns them to the queue.
>
> 5. **Outbox pattern** -- State changes and events are written in the same transaction. A publisher polls the outbox and delivers events. No distributed transactions, no two-phase commit, just PostgreSQL.
>
> 6. **Exactly-once consumer deduplication** -- Consumers track processed event IDs. Redelivery is safe -- duplicate events are silently skipped.
>
> 7. **Full Prometheus observability** -- Every claim, succeed, fail, retry, reclaim, and event publish is a counter. Grafana dashboards give real-time visibility into system health.
>
> All of this runs on a single PostgreSQL instance. No Redis. No Kafka. No external coordinator. The database is the source of truth and the coordination layer."

---

## Step 8: Q&A

Open the floor for questions. Common questions and suggested answers:

**Q: Why PostgreSQL instead of Redis/Kafka for the queue?**
> PostgreSQL gives us ACID transactions. The outbox pattern means state changes and events are atomic. With Redis or Kafka, you need distributed transactions or accept eventual consistency gaps. For this workload, PostgreSQL handles the throughput comfortably.

**Q: What happens if the database goes down?**
> Everything stops gracefully. Workers will fail their health checks and stop claiming. When the database comes back, workers resume. No data is lost because all state is durable in PostgreSQL.

**Q: How does this scale?**
> Horizontally: add more workers. They all compete fairly via SKIP LOCKED. Vertically: PostgreSQL connection pooling (PgBouncer) and read replicas for query traffic. The ceiling is PostgreSQL write throughput, which is substantial for most workloads.

**Q: What about multi-region?**
> This design assumes a single-region PostgreSQL primary. For multi-region, you would need either PostgreSQL logical replication with conflict resolution or a move to a distributed database like CockroachDB.

---

## Cleanup

When the demo is finished, tear down the stack:

```bash
make demo-teardown
```

This stops all containers and removes volumes.

---

## Troubleshooting

### Workers not processing

**Symptoms:** Executions stay in CREATED status, worker logs show errors.

**Fix:**
```bash
docker compose -f docker-compose.yml -f docker-compose.demo.yml logs worker-1
```
Check for:
- `DATABASE_URL` connection errors -- verify PostgreSQL is healthy
- Migration errors -- the `migrate` service may have failed
- OOM kills -- check Docker Desktop resource allocation

### Scenario times out

**Symptoms:** Script prints "Timeout!" before reaching the expected count.

**Fix:**
1. Check if containers are still running:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.demo.yml ps
   ```
2. Look for crashed containers (status `Exited`). Restart them:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.demo.yml up -d
   ```
3. If the issue persists, increase the timeout in the scenario script or reduce the execution count.

### Grafana shows no data

**Symptoms:** Dashboards load but panels say "No data."

**Fix:**
1. Verify Prometheus is scraping targets:
   Open [http://localhost:9091/targets](http://localhost:9091/targets) and confirm all targets show `UP`.
2. If targets are down, check the Prometheus config:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.demo.yml logs prometheus
   ```
3. Ensure the Grafana datasource is configured to point to `http://prometheus:9090` (internal Docker network address).

### Port conflicts

**Symptoms:** `make demo-setup` fails with "address already in use."

**Fix:**
Check what is using the required ports:
```bash
lsof -i :8080   # API
lsof -i :3001   # Grafana
lsof -i :9091   # Prometheus
lsof -i :5432   # PostgreSQL
```
Stop the conflicting process or change the port mapping in `docker-compose.yml`.

### Docker build fails

**Symptoms:** `make demo-setup` fails during the build step.

**Fix:**
1. Ensure Docker Desktop has enough disk space
2. Try a clean build:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.demo.yml build --no-cache
   ```
3. Check that `go.mod` and `go.sum` are up to date:
   ```bash
   go mod tidy
   ```

### Workers restart-looping after scenario 3

**Symptoms:** Workers keep crashing after the retry storm scenario.

**Fix:**
Ensure failure injection was disabled. Restart workers explicitly with `FAILURE_RATE=0`:
```bash
docker compose -f docker-compose.yml -f docker-compose.demo.yml stop worker-1 worker-2 worker-3 worker-4
docker compose -f docker-compose.yml -f docker-compose.demo.yml up -d worker-1 worker-2 worker-3 worker-4
```
