// k6 load test — single stage at 1000 RPS for benchmark report
import http from 'k6/http';
import { check } from 'k6';

const API = __ENV.API_URL || 'http://localhost:8080';
const FUNCTION_ID = __ENV.FUNCTION_ID || '00000000-0000-0000-0000-000000000000';
const DURATION = __ENV.DURATION || '60s';
const RPS = parseInt(__ENV.RPS || '1000');

export const options = {
  scenarios: {
    benchmark: {
      executor: 'ramping-arrival-rate',
      preAllocatedVUs: Math.max(50, RPS / 10),
      timeUnit: '1s',
      stages: [
        { target: RPS, duration: '10s' },
        { target: RPS, duration: DURATION },
        { target: 0,   duration: '10s' },
      ],
    },
  },
  thresholds: {
    http_req_duration: [
      'p(50)<500',
      'p(95)<2000',
      'p(99)<5000',
    ],
    http_req_failed: ['rate<0.05'],
  },
};

export default function () {
  const payload = JSON.stringify({
    input: 'benchmark',
    vu: __VU,
    iter: __ITER,
  });

  const res = http.post(`${API}/f/${FUNCTION_ID}`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(res, { 'success': (r) => r.status === 202 || r.status === 200 });
}

export function handleSummary(data) {
  return {
    stdout: formatBenchmarkSummary(data, RPS, DURATION),
    'benchmark.json': JSON.stringify(data),
  };
}

function formatBenchmarkSummary(data, rps, duration) {
  const m = data.metrics;
  const summary = `
╔══════════════════════════════════════════════╗
║       NimbusRun Benchmark Report             ║
╠══════════════════════════════════════════════╣
║ Target Load: ${String(rps).padStart(6)} RPS for ${duration.padEnd(10)}     ║
╠══════════════════════════════════════════════╣
║ Total Requests:  ${String(m.http_reqs.values.count).padStart(10)}              ║
║ Actual Rate:     ${String(m.http_reqs.values.rate.toFixed(1)).padStart(10)}/s            ║
║ Error Rate:      ${String((m.http_req_failed.values.rate * 100).toFixed(2)).padStart(9)}%             ║
╠══════════════════════════════════════════════╣
║ Latency:                                    ║
║   P50: ${String(m.http_req_duration.values['p(50)'].toFixed(0)).padStart(6)}ms                       ║
║   P95: ${String(m.http_req_duration.values['p(95)'].toFixed(0)).padStart(6)}ms                       ║
║   P99: ${String(m.http_req_duration.values['p(99)'].toFixed(0)).padStart(6)}ms                       ║
║   Max: ${String(m.http_req_duration.values['max'].toFixed(0)).padStart(6)}ms                       ║
╚══════════════════════════════════════════════╝
`;
  return summary;
}
