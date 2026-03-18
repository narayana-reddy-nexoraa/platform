const API_BASE = '/api/v2';

interface FetchOptions {
  method?: string;
  body?: unknown;
  tenantId: string;
}

async function apiFetch<T>(path: string, opts: FetchOptions): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: opts.method || 'GET',
    headers: {
      'Content-Type': 'application/json',
      'X-Tenant-ID': opts.tenantId,
    },
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `API error: ${res.status}`);
  }

  return res.json();
}

// SOP Executions
export interface SOPExecution {
  sop_execution_id: string;
  sop_id: string;
  tenant_id: string;
  industry: string;
  current_step: string;
  status: string;
  temporal_workflow_id?: string;
  started_at: string;
  completed_at?: string;
  created_at: string;
  version: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  total_count: number;
  limit: number;
  offset: number;
}

export function listSOPExecutions(tenantId: string, params?: { sop_id?: string; status?: string; industry?: string; limit?: number; offset?: number }) {
  const qs = new URLSearchParams();
  if (params?.sop_id) qs.set('sop_id', params.sop_id);
  if (params?.status) qs.set('status', params.status);
  if (params?.industry) qs.set('industry', params.industry);
  if (params?.limit) qs.set('limit', String(params.limit));
  if (params?.offset) qs.set('offset', String(params.offset));
  const query = qs.toString() ? `?${qs}` : '';
  return apiFetch<PaginatedResponse<SOPExecution>>(`/sop-executions${query}`, { tenantId });
}

export function getSOPExecution(tenantId: string, id: string) {
  return apiFetch<SOPExecution>(`/sop-executions/${id}`, { tenantId });
}

export function startSOPExecution(tenantId: string, sopId: string, payload: unknown) {
  return apiFetch<SOPExecution>(`/sops/${sopId}/execute`, { tenantId, method: 'POST', body: { payload } });
}

// HITL
export interface HITLRequest {
  request_id: string;
  sop_execution_id: string;
  sop_id: string;
  step_id: string;
  step_name: string;
  decision: string;
  decided_by?: string;
  deadline: string;
  created_at: string;
  is_overdue: boolean;
}

export function listPendingHITL(tenantId: string, limit = 20, offset = 0) {
  return apiFetch<{ data: HITLRequest[]; total_count: number }>(`/hitl/pending?limit=${limit}&offset=${offset}`, { tenantId });
}

export function decideHITL(tenantId: string, requestId: string, decision: string, decidedBy: string, reason: string) {
  return apiFetch<HITLRequest>(`/hitl/${requestId}/decide`, {
    tenantId,
    method: 'POST',
    body: { decision, decided_by: decidedBy, reason },
  });
}

// Audit
export interface AuditEntry {
  audit_id: string;
  step_id: string;
  agent_type: string;
  action: string;
  input_hash: string;
  output_hash: string;
  model_used?: string;
  latency_ms: number;
  created_at: string;
}

export interface AuditTrail {
  sop_execution_id: string;
  entries: AuditEntry[];
  total_entries: number;
}

export function getAuditTrail(tenantId: string, executionId: string) {
  return apiFetch<AuditTrail>(`/audit/executions/${executionId}`, { tenantId });
}
