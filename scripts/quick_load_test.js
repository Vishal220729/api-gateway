import http from 'k6/http';
import { check } from 'k6';

const RATE = parseInt(__ENV.RATE || '2000', 10);
const DURATION = __ENV.DURATION || '10s';

export const options = {
  scenarios: {
    gateway_load: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: Math.min(Math.max(Math.floor(RATE / 10), 50), 500),
      maxVUs: 1000,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<15', 'p(99)<35'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/api/orders`, {
    headers: {
      'X-Client-Id': `client-${__VU}-${__ITER}`,
      'X-Forwarded-For': `10.0.${__VU % 250}.${__ITER % 250}`,
    },
  });

  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    'has rate limit header': (r) => r.headers['X-Ratelimit-Limit'] !== undefined,
  });
}
