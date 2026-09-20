import http from 'k6/http';
import { check, sleep } from 'k6';

// Sustains 10,000 requests/sec against the gateway for 30s using k6's
// constant-arrival-rate executor, which schedules iterations by rate
// rather than by VU count, then auto-scales VUs to keep up.
export const options = {
  scenarios: {
    high_throughput: {
      executor: 'constant-arrival-rate',
      rate: 10000,
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 500,
      maxVUs: 2000,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<5', 'p(99)<15'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/api/orders`, {
    headers: { 'X-Client-Id': `client-${__VU}` },
  });

  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    'has rate limit headers': (r) => r.headers['X-Ratelimit-Limit'] !== undefined,
  });

  sleep(0.001);
}
