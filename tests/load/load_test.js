import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TENANT_ID = __ENV.TENANT_ID || 'a0000000-0000-0000-0000-000000000001';

export const options = {
  scenarios: {
    steady_state: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 100 },  // ramp up
        { duration: '5m', target: 100 },   // steady
        { duration: '30s', target: 0 },    // ramp down
      ],
      gracefulRampDown: '10s',
    },
    peak_burst: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 500 },
        { duration: '60s', target: 500 },
        { duration: '10s', target: 0 },
      ],
      startTime: '7m',  // after steady state
    },
    sustained_backlog: {
      executor: 'shared-iterations',
      vus: 50,
      iterations: 1000,
      startTime: '9m',  // after peak burst
    },
  },
  thresholds: {
    'http_req_duration{scenario:steady_state}': ['p(95)<200'],
    'http_req_failed': ['rate<0.01'],
  },
};

export default function () {
  const idempotencyKey = uuidv4();

  // Create execution
  const createRes = http.post(
    `${BASE_URL}/api/v1/executions`,
    JSON.stringify({ payload: { task: 'load-test', vu: __VU, iter: __ITER } }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-Tenant-ID': TENANT_ID,
        'Idempotency-Key': idempotencyKey,
      },
    }
  );

  check(createRes, {
    'create status is 201': (r) => r.status === 201,
    'has execution_id': (r) => JSON.parse(r.body).execution_id !== undefined,
  });

  // Get execution
  if (createRes.status === 201) {
    const execId = JSON.parse(createRes.body).execution_id;
    const getRes = http.get(
      `${BASE_URL}/api/v1/executions/${execId}`,
      { headers: { 'X-Tenant-ID': TENANT_ID } }
    );
    check(getRes, {
      'get status is 200': (r) => r.status === 200,
    });
  }

  sleep(0.1); // small think time
}
