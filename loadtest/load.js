// k6 load test — 100, 1000, 10000 RPS stages
import http from 'k6/http';
import { check, sleep } from 'k6';

const API = __ENV.API_URL || 'http://localhost:8080';
const FUNCTION_ID = __ENV.FUNCTION_ID || '00000000-0000-0000-0000-000000000000';

export const options = {
  scenarios: {
    load_test: {
      executor: 'ramping-arrival-rate',
      // Warm up at 10 RPS for 10s, then ramp to target
      preAllocatedVUs: 50,
      timeUnit: '1s',
      stages: [
        { target: 10,  duration: '10s' },  // warmup
        { target: 100, duration: '30s' },  // stage 1: 100 RPS
        { target: 0,   duration: '10s' },  // cool down
      ],
    },
    stress_test: {
      executor: 'ramping-arrival-rate',
      preAllocatedVUs: 200,
      timeUnit: '1s',
      stages: [
        { target: 0,   duration: '5s' },
        { target: 1000, duration: '60s' }, // stage 2: 1000 RPS
        { target: 0,   duration: '10s' },
      ],
    },
    spike_test: {
      executor: 'ramping-arrival-rate',
      preAllocatedVUs: 100,
      timeUnit: '1s',
      stages: [
        { target: 0,    duration: '5s' },
        { target: 10000, duration: '30s' }, // spike: 10000 RPS
        { target: 0,    duration: '10s' },
      ],
    },
  },
  thresholds: {
    // Latency targets from the PRD
    http_req_duration: [
      'p(50)<500',   // P50 < 500ms
      'p(95)<2000',  // P95 < 2s
      'p(99)<5000',  // P99 < 5s
    ],
    http_req_failed: ['rate<0.01'], // < 1% error rate
    nimbusrun_queue_depth: ['value<1000'], // queue not growing unbounded
  },
};

export default function () {
  const payload = JSON.stringify({
    input: 'k6-load-test',
    request: __VU,
    iteration: __ITER,
  });

  const res = http.post(`${API}/f/${FUNCTION_ID}`, payload, {
    headers: {
      'Content-Type': 'application/json',
      'X-Request-ID': `${__VU}-${__ITER}`,
    },
    tags: { name: 'invoke' },
  });

  check(res, {
    'status 202 or 503': (r) => r.status === 202 || r.status === 503,
  });

  sleep(0.1);
}

// Summary handler prints the results
export function handleSummary(data) {
  return {
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
    'summary.json': JSON.stringify(data),
  };
}

function textSummary(data, opts) {
  const indent = opts.indent || '';
  const lines = [
    `${indent}=== NimbusRun Load Test Summary ===`,
    `${indent}`,
    `${indent}Total Requests:  ${data.metrics.http_reqs.values.count}`,
    `${indent}Request Rate:    ${data.metrics.http_reqs.values.rate.toFixed(2)}/s`,
    `${indent}Error Rate:      ${(data.metrics.http_req_failed.values.rate * 100).toFixed(2)}%`,
    `${indent}`,
    `${indent}Latency:`,
    `${indent}  P50: ${data.metrics.http_req_duration.values['p(50)']}ms`,
    `${indent}  P95: ${data.metrics.http_req_duration.values['p(95)']}ms`,
    `${indent}  P99: ${data.metrics.http_req_duration.values['p(99)']}ms`,
    `${indent}  Max: ${data.metrics.http_req_duration.values.max}ms`,
  ];

  // Include NimbusRun-specific metrics if available
  if (data.metrics.nimbusrun_queue_depth) {
    lines.push(`${indent}`);
    lines.push(`${indent}Queue Depth: ${data.metrics.nimbusrun_queue_depth.values.value}`);
  }
  if (data.metrics.nimbusrun_cold_starts_total) {
    lines.push(`${indent}Cold Starts: ${data.metrics.nimbusrun_cold_starts_total.values.count}`);
  }

  return lines.join('\n') + '\n';
}
