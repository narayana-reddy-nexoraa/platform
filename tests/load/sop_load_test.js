import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

// Custom metrics
const sopCreated = new Counter('sop_executions_created');
const sopCreateLatency = new Trend('sop_create_latency_ms');
const sopGetLatency = new Trend('sop_get_latency_ms');
const hitlListLatency = new Trend('hitl_list_latency_ms');

export const options = {
  scenarios: {
    sop_executions: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 50 },   // Ramp up to 50 VUs
        { duration: '2m', target: 100 },    // Sustain 100 concurrent
        { duration: '30s', target: 0 },     // Ramp down
      ],
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<2000'],
    sop_create_latency_ms: ['p(95)<1000'],
    sop_get_latency_ms: ['p(95)<100'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:18080';
const TENANT_ID = __ENV.TENANT_ID || '00000000-0000-0000-0000-000000000001';

const SOP_IDS = [
  'FS-01', 'FS-02', 'FS-03', 'FS-04',
  'INS-01', 'INS-02', 'INS-03', 'INS-04',
  'HC-01', 'HC-02', 'HC-03', 'HC-04',
  'HOSP-01', 'HOSP-02', 'HOSP-03', 'HOSP-04',
  'LS-01', 'LS-02', 'LS-03', 'LS-04',
  'MFG-01', 'MFG-02', 'MFG-03', 'MFG-04',
  'CPR-01',
];

const headers = {
  'Content-Type': 'application/json',
  'X-Tenant-ID': TENANT_ID,
};

export default function () {
  const sopId = SOP_IDS[Math.floor(Math.random() * SOP_IDS.length)];

  // 1. Start SOP execution
  const payload = JSON.stringify({
    payload: {
      test: true,
      vu: __VU,
      iter: __ITER,
      sop_id: sopId,
      timestamp: new Date().toISOString(),
    },
  });

  const createRes = http.post(
    `${BASE_URL}/api/v2/sops/${sopId}/execute`,
    payload,
    { headers, tags: { name: 'create_sop' } }
  );

  check(createRes, {
    'create: status is 201': (r) => r.status === 201,
    'create: has execution_id': (r) => {
      try { return JSON.parse(r.body).sop_execution_id !== ''; } catch { return false; }
    },
  });

  if (createRes.status === 201) {
    sopCreated.add(1);
    sopCreateLatency.add(createRes.timings.duration);

    const execId = JSON.parse(createRes.body).sop_execution_id;

    // 2. Get execution by ID
    const getRes = http.get(
      `${BASE_URL}/api/v2/sop-executions/${execId}`,
      { headers, tags: { name: 'get_sop' } }
    );

    check(getRes, {
      'get: status is 200': (r) => r.status === 200,
    });
    sopGetLatency.add(getRes.timings.duration);
  }

  // 3. List pending HITL requests
  const hitlRes = http.get(
    `${BASE_URL}/api/v2/hitl/pending?limit=5`,
    { headers, tags: { name: 'list_hitl' } }
  );

  check(hitlRes, {
    'hitl: status is 200': (r) => r.status === 200,
  });
  hitlListLatency.add(hitlRes.timings.duration);

  sleep(0.5);
}
