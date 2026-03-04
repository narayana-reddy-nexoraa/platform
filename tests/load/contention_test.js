import http from 'k6/http';
import { check } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TENANT_ID = __ENV.TENANT_ID || 'a0000000-0000-0000-0000-000000000001';

export const options = {
  scenarios: {
    seed: {
      executor: 'shared-iterations',
      vus: 10,
      iterations: 50,
      exec: 'seedExecutions',
    },
    contention: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      startTime: '10s', // after seeding
      exec: 'contentionCheck',
    },
  },
  thresholds: {
    'http_req_failed': ['rate<0.01'],
  },
};

export function seedExecutions() {
  const res = http.post(
    `${BASE_URL}/api/v1/executions`,
    JSON.stringify({ payload: { task: 'contention-test', vu: __VU } }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-Tenant-ID': TENANT_ID,
        'Idempotency-Key': uuidv4(),
      },
    }
  );
  check(res, { 'seeded 201': (r) => r.status === 201 });
}

export function contentionCheck() {
  // List executions to observe contention effects
  const res = http.get(
    `${BASE_URL}/api/v1/executions?limit=50`,
    { headers: { 'X-Tenant-ID': TENANT_ID } }
  );
  check(res, {
    'list status 200': (r) => r.status === 200,
    'has data': (r) => JSON.parse(r.body).data !== undefined,
  });
}
