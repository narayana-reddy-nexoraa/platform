---
name: run-sop-test
description: Start an SOP execution via API v2, poll status, and verify audit trail. Usage: /run-sop-test <SOP-ID> [tenant-id]
disable-model-invocation: true
---

# Run SOP Test

Execute an SOP end-to-end and verify the result.

## Arguments

- `$1` — SOP ID (required, e.g., `FS-01`, `INS-01`, `HC-03`)
- `$2` — Tenant ID (optional, defaults to `00000000-0000-0000-0000-000000000001`)

## Steps

1. **Start execution** — POST to `/api/v2/sops/{sop_id}/execute` with a test payload
2. **Poll status** — GET `/api/v2/sop-executions/{id}` every 2 seconds until terminal state (COMPLETED, FAILED, ESCALATED) or 60s timeout
3. **Check audit trail** — GET `/api/v2/audit/executions/{id}` and verify entries exist
4. **Check HITL** — If status is WAITING_HITL, list pending requests and report

## Execution

Use the Bash tool to run curl commands against `http://localhost:8080`. Always include:
- `Content-Type: application/json`
- `X-Tenant-ID: {tenant_id}`

### Start Execution
```bash
curl -s -X POST http://localhost:8080/api/v2/sops/{sop_id}/execute \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: {tenant_id}" \
  -d '{"payload": {"test": true, "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}}'
```

### Poll Status
```bash
curl -s http://localhost:8080/api/v2/sop-executions/{execution_id} \
  -H "X-Tenant-ID: {tenant_id}"
```

### Check Audit Trail
```bash
curl -s http://localhost:8080/api/v2/audit/executions/{execution_id} \
  -H "X-Tenant-ID: {tenant_id}"
```

## Output

Report:
- Execution ID, SOP ID, status, duration
- Number of audit entries (expected: 6 for complete execution)
- Any HITL requests pending
- Pass/fail verdict
