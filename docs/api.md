# API Reference

API documentation for the Narayana execution engine. All endpoints are served via the Gin HTTP framework and require tenant-scoped access.

---

## Common Headers

All endpoints require the following header for multi-tenant isolation:

| Header | Type | Required | Description |
|---|---|---|---|
| `X-Tenant-ID` | UUID | Yes | Identifies the tenant. All queries and mutations are scoped to this tenant. |

---

## Endpoints

### Create Execution

```
POST /api/v1/executions
```

Creates a new execution. This endpoint is **idempotent**: submitting the same `Idempotency-Key` with the same payload will return the previously created execution instead of creating a duplicate.

#### Headers

| Header | Type | Required | Description |
|---|---|---|---|
| `X-Tenant-ID` | UUID | Yes | Tenant isolation key. |
| `Idempotency-Key` | string | Yes | Client-provided key to ensure exactly-once creation. |

#### Request Body

```json
{
  "payload": { ... },
  "max_attempts": 3
}
```

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `payload` | object | Yes | -- | Arbitrary JSON payload for the execution. |
| `max_attempts` | integer | No | 3 | Maximum number of retry attempts before the execution is marked as failed. |

#### Responses

| Status | Condition | Body |
|---|---|---|
| **201 Created** | New execution created (first use of this idempotency key). | `ExecutionResponse` |
| **200 OK** | Idempotent replay -- same key and same payload hash returns the existing execution. | `ExecutionResponse` |
| **409 Conflict** | Same idempotency key but different payload. | `{"error": "idempotency key ... already used with different payload", "code": "IDEMPOTENCY_CONFLICT"}` |
| **400 Bad Request** | Missing `Idempotency-Key` header. | `{"error": "missing required header: Idempotency-Key", "code": "MISSING_HEADER"}` |
| **400 Bad Request** | Malformed or invalid JSON body. | `{"error": "invalid request body: ...", "code": "INVALID_REQUEST"}` |

Payload comparison uses a SHA-256 hash of the canonicalized JSON to detect conflicts.

---

### Get Execution

```
GET /api/v1/executions/:id
```

Returns a single execution by its ID.

#### Headers

| Header | Type | Required | Description |
|---|---|---|---|
| `X-Tenant-ID` | UUID | Yes | Tenant isolation key. |

#### Path Parameters

| Parameter | Type | Description |
|---|---|---|
| `id` | UUID | The execution ID. |

#### Responses

| Status | Condition | Body |
|---|---|---|
| **200 OK** | Execution found. | `ExecutionResponse` |
| **404 Not Found** | No execution with the given ID exists for this tenant. | `{"error": "execution not found: ...", "code": "NOT_FOUND"}` |
| **400 Bad Request** | The `id` path parameter is not a valid UUID. | `{"error": "invalid execution ID format", "code": "INVALID_ID"}` |

---

### List Executions

```
GET /api/v1/executions
```

Returns a paginated list of executions for the tenant, with optional status filtering.

#### Headers

| Header | Type | Required | Description |
|---|---|---|---|
| `X-Tenant-ID` | UUID | Yes | Tenant isolation key. |

#### Query Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `limit` | integer | 20 | Page size. |
| `offset` | integer | 0 | Pagination offset. |
| `status` | string | -- | Filter by execution status. Accepted values: `CREATED`, `CLAIMED`, `RUNNING`, `SUCCEEDED`, `FAILED`, `CANCELED`, `TIMED_OUT`. |

#### Responses

| Status | Condition | Body |
|---|---|---|
| **200 OK** | Success. | `PaginatedResponse` (see below) |
| **400 Bad Request** | Invalid status filter value. | `{"error": "invalid status filter: ...", "code": "INVALID_STATUS"}` |

**PaginatedResponse**

```json
{
  "data": [ "ExecutionResponse", "..." ],
  "total_count": 42,
  "limit": 20,
  "offset": 0
}
```

| Field | Type | Description |
|---|---|---|
| `data` | array | List of `ExecutionResponse` objects. |
| `total_count` | integer | Total number of executions matching the query (across all pages). |
| `limit` | integer | Page size used for this response. |
| `offset` | integer | Offset used for this response. |

---

## Response Schemas

### ExecutionResponse

```json
{
  "execution_id": "UUID",
  "tenant_id": "UUID",
  "idempotency_key": "string",
  "status": "CREATED|CLAIMED|RUNNING|SUCCEEDED|FAILED|CANCELED|TIMED_OUT",
  "attempt_count": 0,
  "max_attempts": 3,
  "locked_by": "worker-id (nullable)",
  "lock_expires_at": "2026-03-04T10:30:00Z (nullable)",
  "error_code": "string (nullable)",
  "error_message": "string (nullable)",
  "payload": {},
  "created_at": "2026-03-04T10:30:00Z",
  "updated_at": "2026-03-04T10:30:00Z",
  "version": 1
}
```

| Field | Type | Nullable | Description |
|---|---|---|---|
| `execution_id` | UUID | No | Unique identifier for the execution. |
| `tenant_id` | UUID | No | Owning tenant. |
| `idempotency_key` | string | No | Client-provided idempotency key. |
| `status` | string | No | Current execution status. One of: `CREATED`, `CLAIMED`, `RUNNING`, `SUCCEEDED`, `FAILED`, `CANCELED`, `TIMED_OUT`. |
| `attempt_count` | integer | No | Number of attempts executed so far. |
| `max_attempts` | integer | No | Maximum allowed attempts. |
| `locked_by` | string | Yes | Identifier of the worker that currently holds the lock. |
| `lock_expires_at` | datetime | Yes | When the current lock expires (ISO 8601). |
| `error_code` | string | Yes | Machine-readable error code from the most recent failure. |
| `error_message` | string | Yes | Human-readable error message from the most recent failure. |
| `payload` | object | No | The JSON payload submitted at creation time. |
| `created_at` | datetime | No | Timestamp when the execution was created (ISO 8601). |
| `updated_at` | datetime | No | Timestamp of the last modification (ISO 8601). |
| `version` | integer | No | Optimistic lock version. Incremented on every mutation. |

### ErrorResponse

```json
{
  "error": "human-readable message",
  "code": "MACHINE_CODE",
  "details": "optional additional info"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `error` | string | Yes | Human-readable error description. |
| `code` | string | Yes | Machine-readable error code (see table below). |
| `details` | string | No | Additional context when available. |

---

## Error Code Mapping

The API maps domain-level errors to HTTP status codes and machine-readable codes as follows:

| Domain Error | HTTP Status | Code |
|---|---|---|
| Not found | 404 | `NOT_FOUND` |
| Idempotency conflict | 409 | `IDEMPOTENCY_CONFLICT` |
| Invalid state transition | 409 | `INVALID_STATE_TRANSITION` |
| Optimistic lock conflict | 409 | `OPTIMISTIC_LOCK_CONFLICT` |
| Validation error | 400 | `VALIDATION_ERROR` |
| Missing header | 400 | `MISSING_HEADER` |
| Claim failed (no work available) | 404 | `NO_WORK_AVAILABLE` |
| Unknown / internal error | 500 | `INTERNAL_ERROR` |

---

## Notes

- All endpoints require the `X-Tenant-ID` header. Requests without it will be rejected.
- Payload comparison for idempotency conflict detection uses a SHA-256 hash of the canonicalized JSON representation.
- All mutations use optimistic locking via the `version` field to prevent concurrent modification. A write that targets a stale version will receive a `409 OPTIMISTIC_LOCK_CONFLICT` response.
