// k6 load test — smoke (100 requests, 1 VU)
import http from 'k6/http';
import { check } from 'k6';

const API = __ENV.API_URL || 'http://localhost:8080';
const FUNCTION_ID = __ENV.FUNCTION_ID || '00000000-0000-0000-0000-000000000000';
const API_KEY = __ENV.API_KEY || '';

export const options = {
  scenarios: {
    smoke: {
      executor: 'constant-vus',
      vus: 1,
      duration: '10s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000'],
    http_req_failed:   ['rate<0.05'],
  },
};

export default function () {
  const payload = JSON.stringify({ input: 'hello' });
  const params  = {
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': API_KEY,
    },
  };
  const res = http.post(`${API}/f/${FUNCTION_ID}`, payload, params);

  check(res, {
    'status accepted': (r) => r.status === 202,
    'has invocation id': (r) => r.body && r.body.includes('id'),
  });
}
