import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

export const options = {
  vus: 50,
  duration: '30s',
  thresholds: {
    http_req_failed: ['rate<0.01'],    // <1% error rate
    http_req_duration: ['p(95)<200'],  // p95 < 200ms
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TENANT_ID = '550e8400-e29b-41d4-a716-446655440000';

export default function () {
  // Create execution with unique idempotency key
  const idempotencyKey = `k6-${__VU}-${__ITER}-${Date.now()}`;
  const payload = JSON.stringify({
    payload: { job_id: idempotencyKey, vu: __VU, iter: __ITER },
    max_attempts: 3,
  });

  const createRes = http.post(`${BASE_URL}/api/v1/executions`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
      'X-Tenant-ID': TENANT_ID,
    },
  });

  check(createRes, {
    'create: status is 201': (r) => r.status === 201,
    'create: has execution_id': (r) => JSON.parse(r.body).execution_id !== undefined,
  });

  if (createRes.status === 201) {
    const execId = JSON.parse(createRes.body).execution_id;

    // Get execution by ID
    const getRes = http.get(`${BASE_URL}/api/v1/executions/${execId}`, {
      headers: { 'X-Tenant-ID': TENANT_ID },
    });

    check(getRes, {
      'get: status is 200': (r) => r.status === 200,
      'get: correct id': (r) => JSON.parse(r.body).execution_id === execId,
    });
  }

  // List executions
  const listRes = http.get(`${BASE_URL}/api/v1/executions?limit=10`, {
    headers: { 'X-Tenant-ID': TENANT_ID },
  });

  check(listRes, {
    'list: status is 200': (r) => r.status === 200,
    'list: has data array': (r) => Array.isArray(JSON.parse(r.body).data),
  });

  // Test idempotency — resend same key
  const dupeRes = http.post(`${BASE_URL}/api/v1/executions`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
      'X-Tenant-ID': TENANT_ID,
    },
  });

  check(dupeRes, {
    'idempotent: status is 200': (r) => r.status === 200,
  });

  sleep(0.1);
}
