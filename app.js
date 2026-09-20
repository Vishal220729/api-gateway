/**
 * API Gateway Web Dashboard - Real-time Controller
 * Black & Neon Light Green Theme
 * Ultimate Enterprise Suite:
 * 1. SSE Real-Time Stream (Zero Polling)
 * 2. Chaos Engineering & Fault Injection
 * 3. GeoIP Country-Level Firewall
 * 4. Interactive Request Architecture Flow Animation
 * 5. Multi-Node Load Balancer with Active Health Checks
 * 6. Tier-Based API Key Manager (Free, Pro, Enterprise VIP)
 * 7. Redis HTTP Response Caching (X-Cache: HIT / MISS)
 * 8. Distributed Request Tracing with Waterfall Modal
 * 9. 1-Click Traffic Stream Export (CSV & JSON)
 * 10. WAF IP Firewall & Circuit Breaker Controller
 */

// State
const state = {
  routes: [],
  previousTotalRequests: 0,
  previousTimestamp: Date.now(),
  historyRPS: new Array(30).fill(0),
  historyThrottled: new Array(30).fill(0),
  selectedRoute: {
    path: '/api/orders',
    method: 'GET',
    requireAuth: false,
    limit: 100,
  },
  lastToken: '',
  trafficFilter: 'ALL',
  trafficLogs: [],
  wafRules: [],
  circuits: [],
  upstreamNodes: [],
  apiKeys: [],
  geoIPRules: [],
  chaosConfig: {
    enabled: false,
    latency_ms: 0,
    fault_rate_pct: 0,
  },
};

// DOM Elements
const elTotalReqs = document.getElementById('val-total-requests');
const elRateLimited = document.getElementById('val-rate-limited');
const elAvgLatency = document.getElementById('val-avg-latency');
const elActiveBuckets = document.getElementById('val-active-buckets');
const elRps = document.getElementById('val-rps');
const elRedisBadge = document.getElementById('redis-status-badge');
const elSseBadge = document.getElementById('sse-status-badge');
const elBtnFlushBuckets = document.getElementById('btn-flush-buckets');
const elBtnOpenRouteModal = document.getElementById('btn-open-route-modal');

// Traffic Stream DOM
const elTrafficLogTbody = document.getElementById('traffic-log-tbody');
const elLogFilterPills = document.getElementById('log-filter-pills');
const elBtnExportCSV = document.getElementById('btn-export-csv');
const elBtnExportJSON = document.getElementById('btn-export-json');

// Playground DOM
const elRouteTabs = document.getElementById('route-tabs');
const elTargetUrl = document.getElementById('input-target-url');
const elClientIp = document.getElementById('input-client-ip');
const elClientId = document.getElementById('input-client-id');
const elJwtToken = document.getElementById('input-jwt-token');
const elApiKey = document.getElementById('input-api-key');
const elJwtContainer = document.getElementById('jwt-token-container');
const elBtnMint = document.getElementById('btn-mint-token');
const elBtnTierFree = document.getElementById('btn-tier-free');
const elBtnTierPro = document.getElementById('btn-tier-pro');
const elBtnTierVip = document.getElementById('btn-tier-vip');
const elBtnSendSingle = document.getElementById('btn-send-single');
const elBtnBurst10 = document.getElementById('btn-send-burst-10');
const elBtnBurst55 = document.getElementById('btn-send-burst-55');
const elBtnBenchmark = document.getElementById('btn-benchmark-quick');
const elResBadge = document.getElementById('res-status-badge');
const elResTime = document.getElementById('res-time');
const elLimitRemaining = document.getElementById('res-limit-remaining');
const elLimitTotal = document.getElementById('res-limit-total');
const elMeterFill = document.getElementById('meter-bar-fill');
const elResBodyCode = document.getElementById('res-body-code');
const elResHeadersCode = document.getElementById('res-headers-code');

// Chaos DOM
const elValChaosLatency = document.getElementById('val-chaos-latency');
const elSliderChaosLatency = document.getElementById('slider-chaos-latency');
const elValChaosFault = document.getElementById('val-chaos-fault');
const elFaultBtnGroup = document.getElementById('fault-btn-group');
const elBtnApplyChaos = document.getElementById('btn-apply-chaos');
const elBtnResetChaos = document.getElementById('btn-reset-chaos');

// GeoIP DOM
const elInputGeoipCountry = document.getElementById('input-geoip-country');
const elSelectGeoipAction = document.getElementById('select-geoip-action');
const elInputGeoipReason = document.getElementById('input-geoip-reason');
const elBtnAddGeoipRule = document.getElementById('btn-add-geoip-rule');
const elGeoipTbody = document.getElementById('geoip-tbody');

// Right Stack DOM
const elNodesList = document.getElementById('nodes-list');
const elApikeysTbody = document.getElementById('apikeys-tbody');
const elInputKeyOwner = document.getElementById('input-key-owner');
const elSelectKeyTier = document.getElementById('select-key-tier');
const elBtnGenerateApikey = document.getElementById('btn-generate-apikey');
const elCircuitsList = document.getElementById('circuits-list');
const elInputWafIp = document.getElementById('input-waf-ip');
const elSelectWafType = document.getElementById('select-waf-type');
const elInputWafReason = document.getElementById('input-waf-reason');
const elBtnAddWafRule = document.getElementById('btn-add-waf-rule');
const elWafTbody = document.getElementById('waf-tbody');
const elRouteCardsList = document.getElementById('route-cards-list');
const elBucketsTbody = document.getElementById('buckets-tbody');
const elToast = document.getElementById('toast');
const canvas = document.getElementById('traffic-canvas');
const ctx = canvas ? canvas.getContext('2d') : null;

// Route Modal DOM
const elRouteModal = document.getElementById('route-modal');
const elBtnCloseRouteModal = document.getElementById('btn-close-route-modal');
const elBtnCancelRouteModal = document.getElementById('btn-cancel-route-modal');
const elBtnSubmitNewRoute = document.getElementById('btn-submit-new-route');
const elNewRoutePath = document.getElementById('new-route-path');
const elNewRouteUpstream = document.getElementById('new-route-upstream');
const elNewRouteLimit = document.getElementById('new-route-limit');
const elNewRouteWindow = document.getElementById('new-route-window');
const elNewRouteKeytype = document.getElementById('new-route-keytype');
const elNewRouteAuth = document.getElementById('new-route-auth');

// Trace Modal DOM
const elTraceModal = document.getElementById('trace-modal');
const elBtnCloseTraceModal = document.getElementById('btn-close-trace-modal');
const elBtnDismissTraceModal = document.getElementById('btn-dismiss-trace-modal');
const elTraceModalTitle = document.getElementById('trace-modal-title');
const elTraceModalBody = document.getElementById('trace-modal-body');

// Toast Notification
function showToast(msg, duration = 3000) {
  elToast.textContent = msg;
  elToast.classList.add('show');
  setTimeout(() => elToast.classList.remove('show'), duration);
}

function formatNum(num) {
  return Number(num || 0).toLocaleString();
}

function formatTime(isoString) {
  if (!isoString) return '--:--:--';
  const d = new Date(isoString);
  return d.toTimeString().split(' ')[0] + '.' + String(d.getMilliseconds()).padStart(3, '0');
}

// -------------------------------------------------------------
// 1. Server-Sent Events (SSE) Real-Time Connection
// -------------------------------------------------------------
function initSSE() {
  try {
    const sse = new EventSource('/api/dashboard/stream');

    sse.addEventListener('connected', () => {
      if (elSseBadge) {
        elSseBadge.innerHTML = '<span class="pulse-dot green"></span><span>SSE LIVE</span>';
        elSseBadge.style.borderColor = 'rgba(0, 255, 136, 0.4)';
      }
    });

    sse.addEventListener('traffic', e => {
      try {
        const log = JSON.parse(e.data);
        state.trafficLogs.unshift(log);
        if (state.trafficLogs.length > 50) {
          state.trafficLogs.pop();
        }
        renderTrafficLogs();
      } catch (err) {
        console.warn('Error parsing SSE traffic log:', err);
      }
    });

    sse.onerror = () => {
      if (elSseBadge) {
        elSseBadge.innerHTML = '<span class="pulse-dot yellow"></span><span>SSE RETRY</span>';
        elSseBadge.style.borderColor = 'rgba(250, 204, 21, 0.4)';
      }
    };
  } catch (err) {
    console.warn('SSE initialization error:', err);
  }
}

// -------------------------------------------------------------
// 2. Fetch live gateway statistics & charts
// -------------------------------------------------------------
async function fetchStats() {
  try {
    const res = await fetch('/api/dashboard/stats');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();

    if (elTotalReqs) elTotalReqs.textContent = formatNum(data.total_requests);
    if (elRateLimited) elRateLimited.textContent = formatNum(data.total_rate_limited);
    if (elAvgLatency) elAvgLatency.textContent = data.avg_latency || '< 1 ms';
    if (elActiveBuckets) elActiveBuckets.textContent = formatNum(data.active_buckets_count);

    const now = Date.now();
    const elapsedSec = Math.max((now - state.previousTimestamp) / 1000, 0.5);
    const diffRequests = Math.max(0, data.total_requests - state.previousTotalRequests);
    const currentRps = Math.round(diffRequests / elapsedSec);

    state.previousTotalRequests = data.total_requests;
    state.previousTimestamp = now;
    if (elRps) elRps.textContent = `+${formatNum(currentRps)} req/s`;

    state.historyRPS.push(currentRps);
    state.historyRPS.shift();
    state.historyThrottled.push(data.total_rate_limited);
    state.historyThrottled.shift();

    drawChart();

    if (data.routes && JSON.stringify(data.routes) !== JSON.stringify(state.routes)) {
      state.routes = data.routes;
      renderRoutes(data.routes);
      renderRouteTabs(data.routes);
    }

    renderBuckets(data.active_buckets || []);

    if (elRedisBadge) {
      if (data.redis_connected) {
        elRedisBadge.innerHTML = '<span class="pulse-dot green"></span><span>REDIS CONNECTED</span><span class="port-tag">:6379</span>';
        elRedisBadge.style.borderColor = 'rgba(0, 255, 136, 0.3)';
      } else {
        elRedisBadge.innerHTML = '<span class="pulse-dot" style="background:#f43f5e;box-shadow:none;"></span><span>REDIS OFFLINE</span>';
        elRedisBadge.style.borderColor = 'rgba(244, 63, 94, 0.5)';
      }
    }
  } catch (err) {
    console.warn('Failed to fetch stats:', err);
  }
}

function drawChart() {
  if (!ctx || !canvas) return;
  const w = canvas.width;
  const h = canvas.height;

  ctx.clearRect(0, 0, w, h);

  ctx.strokeStyle = 'rgba(0, 255, 136, 0.05)';
  ctx.lineWidth = 1;
  const gridRows = 4;
  for (let i = 1; i < gridRows; i++) {
    const y = (h / gridRows) * i;
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(w, y);
    ctx.stroke();
  }

  const maxVal = Math.max(...state.historyRPS, 50);
  const points = state.historyRPS.length;
  const stepX = w / (points - 1);

  const grad = ctx.createLinearGradient(0, 0, 0, h);
  grad.addColorStop(0, 'rgba(0, 255, 136, 0.35)');
  grad.addColorStop(1, 'rgba(0, 255, 136, 0.0)');

  ctx.beginPath();
  state.historyRPS.forEach((val, i) => {
    const x = i * stepX;
    const y = h - (val / maxVal) * (h - 20) - 10;
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  });
  ctx.lineTo(w, h);
  ctx.lineTo(0, h);
  ctx.closePath();
  ctx.fillStyle = grad;
  ctx.fill();

  ctx.beginPath();
  state.historyRPS.forEach((val, i) => {
    const x = i * stepX;
    const y = h - (val / maxVal) * (h - 20) - 10;
    if (i === 0) ctx.moveTo(x, y);
    else ctx.lineTo(x, y);
  });
  ctx.strokeStyle = '#00ff88';
  ctx.lineWidth = 2.5;
  ctx.shadowColor = '#00ff88';
  ctx.shadowBlur = 10;
  ctx.stroke();
  ctx.shadowBlur = 0;

  state.historyRPS.forEach((val, i) => {
    const x = i * stepX;
    const y = h - (val / maxVal) * (h - 20) - 10;
    ctx.beginPath();
    ctx.arc(x, y, 3, 0, Math.PI * 2);
    ctx.fillStyle = '#03150b';
    ctx.strokeStyle = '#00ff88';
    ctx.lineWidth = 2;
    ctx.fill();
    ctx.stroke();
  });
}

// -------------------------------------------------------------
// 3. Live Traffic Stream (Real-Time Logger)
// -------------------------------------------------------------
async function fetchTrafficLogs() {
  try {
    const res = await fetch('/api/dashboard/traffic');
    if (!res.ok) return;
    const logs = await res.json();
    state.trafficLogs = logs || [];
    renderTrafficLogs();
  } catch (err) {
    console.warn('Failed to fetch traffic stream:', err);
  }
}

function renderTrafficLogs() {
  if (!elTrafficLogTbody) return;
  const filter = state.trafficFilter;
  const filtered = (state.trafficLogs || []).filter(item => {
    if (filter === 'ALL') return true;
    if (filter === '200') return item.status === 200;
    if (filter === '429') return item.status === 429;
    if (filter === 'BLOCKED') return item.status === 401 || item.status === 403 || item.status === 500 || item.status === 503;
    return true;
  });

  if (filtered.length === 0) {
    elTrafficLogTbody.innerHTML = `
      <tr>
        <td colspan="9" class="empty-cell">Listening for live traffic... Send a request to see logs stream in real time.</td>
      </tr>
    `;
    return;
  }

  elTrafficLogTbody.innerHTML = filtered.map((log, idx) => {
    let statusClass = 'log-200';
    if (log.status === 429) statusClass = 'log-429';
    else if (log.status === 401 || log.status === 403 || log.status === 500 || log.status === 503) statusClass = `log-${log.status}`;

    const remainingStr = log.remaining !== undefined && log.limit !== undefined 
      ? `${log.remaining} / ${log.limit}` 
      : '--';

    let cacheTag = '<span class="cache-tag cache-miss">MISS</span>';
    if (log.cache_status === 'HIT') {
      cacheTag = '<span class="cache-tag cache-hit">⚡ HIT</span>';
    }

    return `
      <tr>
        <td class="mono" style="color:var(--text-muted);font-size:0.72rem;">${formatTime(log.timestamp)}</td>
        <td><span class="method-tag ${log.method ? log.method.toLowerCase() : 'get'}">${log.method || 'GET'}</span></td>
        <td class="mono" style="color:#ffffff;font-weight:600;">${log.path}</td>
        <td class="mono" style="color:var(--accent-mint);">${log.client_ip || '127.0.0.1'}</td>
        <td><span class="log-status-badge ${statusClass}">${log.status}</span></td>
        <td>${cacheTag}</td>
        <td class="mono">${log.duration_ms} ms</td>
        <td class="mono">${remainingStr}</td>
        <td>
          <button class="btn btn-tiny btn-ghost" style="padding:2px 6px;font-size:0.68rem;" onclick="openTraceModalByIndex(${idx})">
            🔍 Trace
          </button>
        </td>
      </tr>
    `;
  }).join('');
}

// -------------------------------------------------------------
// 4. Chaos Engineering Controller
// -------------------------------------------------------------
async function fetchChaosConfig() {
  try {
    const res = await fetch('/api/dashboard/chaos');
    if (!res.ok) return;
    state.chaosConfig = await res.json();
    if (elSliderChaosLatency) elSliderChaosLatency.value = state.chaosConfig.latency_ms || 0;
    if (elValChaosLatency) elValChaosLatency.textContent = `${state.chaosConfig.latency_ms || 0} ms`;
    if (elValChaosFault) elValChaosFault.textContent = `${state.chaosConfig.fault_rate_pct || 0}%`;

    if (elFaultBtnGroup) {
      elFaultBtnGroup.querySelectorAll('.pill-fault').forEach(b => {
        if (parseInt(b.dataset.fault, 10) === (state.chaosConfig.fault_rate_pct || 0)) {
          b.classList.add('active');
        } else {
          b.classList.remove('active');
        }
      });
    }
  } catch (err) {
    console.warn('Failed to fetch chaos config:', err);
  }
}

async function applyChaosSettings() {
  const latency = parseInt(elSliderChaosLatency.value, 10) || 0;
  let fault = 0;
  const activeFaultBtn = elFaultBtnGroup ? elFaultBtnGroup.querySelector('.pill-fault.active') : null;
  if (activeFaultBtn) {
    fault = parseInt(activeFaultBtn.dataset.fault, 10) || 0;
  }

  const enabled = latency > 0 || fault > 0;
  const newCfg = {
    enabled,
    latency_ms: latency,
    fault_rate_pct: fault
  };

  try {
    elBtnApplyChaos.disabled = true;
    const res = await fetch('/api/dashboard/chaos', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newCfg)
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    state.chaosConfig = newCfg;
    showToast(enabled ? `⚡ Chaos Engineering Active: +${latency}ms delay, ${fault}% fault rate!` : '🟢 Chaos Engineering Disabled');
  } catch (err) {
    showToast(`❌ Error setting chaos: ${err.message}`);
  } finally {
    elBtnApplyChaos.disabled = false;
  }
}

async function resetChaosSettings() {
  if (elSliderChaosLatency) elSliderChaosLatency.value = 0;
  if (elValChaosLatency) elValChaosLatency.textContent = '0 ms';
  if (elValChaosFault) elValChaosFault.textContent = '0%';
  if (elFaultBtnGroup) {
    elFaultBtnGroup.querySelectorAll('.pill-fault').forEach(b => {
      if (b.dataset.fault === '0') b.classList.add('active');
      else b.classList.remove('active');
    });
  }
  await applyChaosSettings();
}

// -------------------------------------------------------------
// 5. GeoIP Country-Level Firewall
// -------------------------------------------------------------
async function fetchGeoIPRules() {
  try {
    const res = await fetch('/api/dashboard/geoip');
    if (!res.ok) return;
    state.geoIPRules = await res.json() || [];
    renderGeoIPRules();
  } catch (err) {
    console.warn('Failed to fetch GeoIP rules:', err);
  }
}

function renderGeoIPRules() {
  if (!elGeoipTbody) return;
  if (state.geoIPRules.length === 0) {
    elGeoipTbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No GeoIP country rules defined. Add one above.</td></tr>`;
    return;
  }

  elGeoipTbody.innerHTML = state.geoIPRules.map(r => `
    <tr>
      <td class="mono" style="color:#ffffff;font-weight:700;">${r.country_code}</td>
      <td><span class="waf-tag ${r.action === 'block' ? 'blacklist' : 'whitelist'}">${r.action.toUpperCase()}</span></td>
      <td style="color:var(--text-muted);font-size:0.75rem;">${r.reason || 'Regional rule'}</td>
      <td>
        <button class="btn btn-tiny btn-ghost" style="color:var(--accent-red);padding:2px 6px;" onclick="removeGeoIPRule('${r.country_code}')">
          ✖
        </button>
      </td>
    </tr>
  `).join('');
}

async function addGeoIPRule() {
  const country = elInputGeoipCountry.value.trim().toUpperCase();
  const action = elSelectGeoipAction.value;
  const reason = elInputGeoipReason.value.trim() || 'Regional security policy';

  if (!country || country.length !== 2) {
    showToast('⚠️ Please enter a 2-letter ISO country code (e.g. RU, CN)');
    return;
  }

  try {
    elBtnAddGeoipRule.disabled = true;
    const res = await fetch('/api/dashboard/geoip', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ country_code: country, action, reason })
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🌍 Added GeoIP rule: ${country} -> ${action.toUpperCase()}`);
    elInputGeoipCountry.value = '';
    elInputGeoipReason.value = '';
    await fetchGeoIPRules();
  } catch (err) {
    showToast(`❌ Error adding GeoIP rule: ${err.message}`);
  } finally {
    elBtnAddGeoipRule.disabled = false;
  }
}

window.removeGeoIPRule = async function(country) {
  try {
    const res = await fetch(`/api/dashboard/geoip?country=${encodeURIComponent(country)}`, {
      method: 'DELETE'
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🗑️ Removed GeoIP rule for ${country}`);
    await fetchGeoIPRules();
  } catch (err) {
    showToast(`❌ Error removing GeoIP rule: ${err.message}`);
  }
};

// -------------------------------------------------------------
// 6. Interactive Architecture Pipeline Animation
// -------------------------------------------------------------
function animatePipeline() {
  const stages = [
    'stage-client',
    'stage-waf',
    'stage-security',
    'stage-apikey',
    'stage-limiter',
    'stage-cache',
    'stage-circuit',
    'stage-upstream'
  ];

  stages.forEach((id, i) => {
    setTimeout(() => {
      const el = document.getElementById(id);
      if (el) {
        el.classList.add('pulse');
        setTimeout(() => el.classList.remove('pulse'), 400);
      }
    }, i * 60);
  });
}

// -------------------------------------------------------------
// 7. Multi-Node Load Balancer & Active Health Checks
// -------------------------------------------------------------
async function fetchUpstreamNodes() {
  try {
    const res = await fetch('/api/dashboard/upstreams');
    if (!res.ok) return;
    state.upstreamNodes = await res.json() || [];
    renderUpstreamNodes();
  } catch (err) {
    console.warn('Failed to fetch upstreams:', err);
  }
}

function renderUpstreamNodes() {
  if (!elNodesList) return;
  if (state.upstreamNodes.length === 0) {
    elNodesList.innerHTML = `<div class="empty-cell" style="padding:10px;text-align:center;">No backend nodes registered.</div>`;
    return;
  }

  elNodesList.innerHTML = state.upstreamNodes.map(node => {
    const isHealthy = node.healthy;
    const badgeClass = isHealthy ? 'node-healthy' : 'node-unhealthy';
    const dotClass = isHealthy ? 'pulse-dot green' : 'pulse-dot' /* red */;
    const label = isHealthy ? 'HEALTHY' : 'DOWN';

    return `
      <div class="node-card">
        <div class="node-info">
          <span class="node-target">${node.url}</span>
          <span class="node-meta">Ping: <strong class="mono" style="color:#00ff88;">${node.latency_ms}ms</strong> | Conns: <strong class="mono" style="color:#ffffff;">${node.active_conns}</strong> | Failures: ${node.fails}</span>
        </div>
        <span class="node-status-badge ${badgeClass}">
          <span class="${dotClass}"></span>
          ${label}
        </span>
      </div>
    `;
  }).join('');
}

// -------------------------------------------------------------
// 8. API Key Management & Tier Quotas
// -------------------------------------------------------------
async function fetchAPIKeys() {
  try {
    const res = await fetch('/api/dashboard/apikeys');
    if (!res.ok) return;
    state.apiKeys = await res.json() || [];
    renderAPIKeys();
  } catch (err) {
    console.warn('Failed to fetch API keys:', err);
  }
}

function renderAPIKeys() {
  if (!elApikeysTbody) return;
  if (state.apiKeys.length === 0) {
    elApikeysTbody.innerHTML = `<tr><td colspan="5" class="empty-cell">No API keys registered. Mint one above.</td></tr>`;
    return;
  }

  elApikeysTbody.innerHTML = state.apiKeys.map(k => {
    let tierClass = 'tier-free';
    if (k.tier === 'pro') tierClass = 'tier-pro';
    else if (k.tier === 'enterprise') tierClass = 'tier-enterprise';

    return `
      <tr>
        <td class="mono" style="color:#00ff88;font-size:0.72rem;" title="${k.key}">${k.key.substring(0, 16)}...</td>
        <td style="color:#ffffff;font-size:0.74rem;">${k.owner}</td>
        <td><span class="apikey-tier-badge ${tierClass}">${k.tier}</span></td>
        <td class="mono" style="font-size:0.72rem;">${k.rate_limit}/min</td>
        <td>
          <button class="btn btn-tiny btn-ghost" style="color:var(--accent-red);padding:2px 6px;" onclick="revokeAPIKey('${k.key}')">
            ✖ Revoke
          </button>
        </td>
      </tr>
    `;
  }).join('');
}

async function generateAPIKey() {
  const owner = elInputKeyOwner.value.trim() || 'Internal App';
  const tier = elSelectKeyTier.value;

  try {
    elBtnGenerateApikey.disabled = true;
    const res = await fetch('/api/dashboard/apikeys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ owner, tier })
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const keyData = await res.json();
    showToast(`⚡ Generated ${tier.toUpperCase()} key for "${owner}"!`);
    elInputKeyOwner.value = '';
    
    if (elApiKey) elApiKey.value = keyData.key;
    await fetchAPIKeys();
  } catch (err) {
    showToast(`❌ Error generating key: ${err.message}`);
  } finally {
    elBtnGenerateApikey.disabled = false;
  }
}

window.revokeAPIKey = async function(key) {
  if (!confirm(`Are you sure you want to revoke API key ${key.substring(0, 16)}...?`)) return;
  try {
    const res = await fetch(`/api/dashboard/apikeys?key=${encodeURIComponent(key)}`, {
      method: 'DELETE'
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🗑️ Revoked API key`);
    await fetchAPIKeys();
  } catch (err) {
    showToast(`❌ Error revoking key: ${err.message}`);
  }
};

// -------------------------------------------------------------
// 9. Distributed Tracing & Waterfall Modal
// -------------------------------------------------------------
window.openTraceModalByIndex = function(idx) {
  const filtered = (state.trafficLogs || []).filter(item => {
    if (state.trafficFilter === 'ALL') return true;
    if (state.trafficFilter === '200') return item.status === 200;
    if (state.trafficFilter === '429') return item.status === 429;
    if (state.trafficFilter === 'BLOCKED') return item.status === 401 || item.status === 403 || item.status === 500 || item.status === 503;
    return true;
  });

  const log = filtered[idx];
  if (!log) return;

  const total = Math.max(log.duration_ms, 1);
  const wafEstimate = Math.min(0.1, total * 0.05);
  const limiterEstimate = Math.min(0.4, total * 0.15);
  const upstreamEstimate = Math.max(0.1, total - (wafEstimate + limiterEstimate));

  const wafPct = Math.round((wafEstimate / total) * 100);
  const limiterPct = Math.round((limiterEstimate / total) * 100);
  const upstreamPct = Math.round((upstreamEstimate / total) * 100);

  elTraceModalTitle.textContent = `Trace: ${log.correlation_id || 'req_live_' + log.id}`;
  elTraceModalBody.innerHTML = `
    <div class="trace-waterfall">
      <div class="trace-meta-box">
        <div>Path: <strong class="mono" style="color:#00ff88;">${log.method} ${log.path}</strong></div>
        <div>Status: <strong class="mono" style="color:#ffffff;">${log.status}</strong></div>
        <div>Total Latency: <strong class="mono" style="color:#00ff88;">${log.duration_ms} ms</strong></div>
        <div>Cache: <strong class="mono">${log.cache_status || 'MISS'}</strong></div>
      </div>

      <div class="waterfall-step">
        <div class="step-label-row">
          <span>1. WAF &amp; GeoIP Inspection</span>
          <strong>~${wafEstimate.toFixed(1)} ms</strong>
        </div>
        <div class="step-bar-track">
          <div class="step-bar-fill fill-waf" style="width: ${Math.max(5, wafPct)}%;"></div>
        </div>
      </div>

      <div class="waterfall-step">
        <div class="step-label-row">
          <span>2. Sliding-Window Limiter (Redis Lua EVALSHA)</span>
          <strong>~${limiterEstimate.toFixed(1)} ms</strong>
        </div>
        <div class="step-bar-track">
          <div class="step-bar-fill fill-limiter" style="width: ${Math.max(5, limiterPct)}%;"></div>
        </div>
      </div>

      <div class="waterfall-step">
        <div class="step-label-row">
          <span>3. Reverse Proxy &amp; Upstream Round-Trip</span>
          <strong>~${upstreamEstimate.toFixed(1)} ms</strong>
        </div>
        <div class="step-bar-track">
          <div class="step-bar-fill fill-upstream" style="width: ${Math.max(5, upstreamPct)}%;"></div>
        </div>
      </div>

      <div style="background:#020403;border:1px solid rgba(0,255,136,0.1);border-radius:6px;padding:10px;font-size:0.75rem;font-family:var(--font-mono);color:var(--text-muted);display:flex;flex-direction:column;gap:4px;">
        <div>X-Correlation-ID: <span style="color:#ffffff;">${log.correlation_id || '--'}</span></div>
        <div>Client IP: <span style="color:#22d3ee;">${log.client_ip}</span></div>
        <div>RateLimit Remaining: <span style="color:#00ff88;">${log.remaining !== undefined ? log.remaining : '--'}</span> / ${log.limit || '--'}</div>
        <div>Tier: <span style="color:#facc15;">${log.tier || 'Standard Route Limit'}</span></div>
      </div>
    </div>
  `;

  elTraceModal.classList.add('open');
};

function closeTraceModal() {
  elTraceModal.classList.remove('open');
}

// -------------------------------------------------------------
// 10. 1-Click Traffic Stream Export (CSV & JSON)
// -------------------------------------------------------------
function exportTrafficCSV() {
  if (!state.trafficLogs || state.trafficLogs.length === 0) {
    showToast('⚠️ No traffic logs to export');
    return;
  }

  const headers = ['id', 'correlation_id', 'timestamp', 'method', 'path', 'client_ip', 'status', 'duration_ms', 'limit', 'remaining', 'cache_status', 'tier'];
  const rows = state.trafficLogs.map(l => [
    l.id,
    `"${l.correlation_id || ''}"`,
    `"${l.timestamp}"`,
    `"${l.method}"`,
    `"${l.path}"`,
    `"${l.client_ip}"`,
    l.status,
    l.duration_ms,
    l.limit || 0,
    l.remaining || 0,
    `"${l.cache_status || 'MISS'}"`,
    `"${l.tier || ''}"`
  ]);

  const csvContent = 'data:text/csv;charset=utf-8,' + [headers.join(','), ...rows.map(r => r.join(','))].join('\n');
  const encodedUri = encodeURI(csvContent);
  const link = document.createElement('a');
  link.setAttribute('href', encodedUri);
  link.setAttribute('download', `gateway_traffic_${Date.now()}.csv`);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  showToast('📥 Exported traffic logs to CSV!');
}

function exportTrafficJSON() {
  if (!state.trafficLogs || state.trafficLogs.length === 0) {
    showToast('⚠️ No traffic logs to export');
    return;
  }

  const jsonStr = 'data:text/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(state.trafficLogs, null, 2));
  const link = document.createElement('a');
  link.setAttribute('href', jsonStr);
  link.setAttribute('download', `gateway_traffic_${Date.now()}.json`);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  showToast('📥 Exported traffic logs to JSON!');
}

// -------------------------------------------------------------
// 11. Redis Rate Limit Bucket Flush
// -------------------------------------------------------------
async function flushRedisBuckets() {
  if (!confirm('Are you sure you want to flush all active rate limit buckets in Redis? Counters will immediately reset.')) {
    return;
  }
  try {
    elBtnFlushBuckets.disabled = true;
    elBtnFlushBuckets.textContent = 'Flushing...';
    const res = await fetch('/api/dashboard/reset-buckets', { method: 'POST' });
    const data = await res.json();
    showToast(`⚡ Flushed ${data.deleted || 0} active Redis rate-limiting bucket(s)!`);
    await fetchStats();
  } catch (err) {
    showToast(`❌ Error flushing buckets: ${err.message}`);
  } finally {
    elBtnFlushBuckets.disabled = false;
    elBtnFlushBuckets.textContent = '⚡ Flush Buckets';
  }
}

// -------------------------------------------------------------
// 12. WAF IP Firewall Rules
// -------------------------------------------------------------
async function fetchWAFRules() {
  try {
    const res = await fetch('/api/dashboard/waf');
    if (!res.ok) return;
    state.wafRules = await res.json() || [];
    renderWAFRules();
  } catch (err) {
    console.warn('Failed to fetch WAF rules:', err);
  }
}

function renderWAFRules() {
  if (!elWafTbody) return;
  if (state.wafRules.length === 0) {
    elWafTbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No active WAF rules. Add IP above to blacklist or whitelist.</td></tr>`;
    return;
  }

  elWafTbody.innerHTML = state.wafRules.map(r => `
    <tr>
      <td class="mono" style="color:#ffffff;font-weight:600;">${r.ip}</td>
      <td><span class="waf-tag ${r.type}">${r.type.toUpperCase()}</span></td>
      <td style="color:var(--text-muted);font-size:0.75rem;">${r.reason || 'Manual rule'}</td>
      <td>
        <button class="btn btn-tiny btn-ghost" style="color:var(--accent-red);padding:2px 8px;" onclick="deleteWAFRule('${r.ip}')">
          ✖ Remove
        </button>
      </td>
    </tr>
  `).join('');
}

async function addWAFRule() {
  const ip = elInputWafIp.value.trim();
  const type = elSelectWafType.value;
  const reason = elInputWafReason.value.trim() || 'Manual dashboard rule';

  if (!ip) {
    showToast('⚠️ Please enter a valid IP address');
    return;
  }

  try {
    elBtnAddWafRule.disabled = true;
    const res = await fetch('/api/dashboard/waf', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ip, type, reason })
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🛡️ Added WAF rule: ${ip} -> ${type.toUpperCase()}`);
    elInputWafIp.value = '';
    elInputWafReason.value = '';
    await fetchWAFRules();
  } catch (err) {
    showToast(`❌ Error adding WAF rule: ${err.message}`);
  } finally {
    elBtnAddWafRule.disabled = false;
  }
}

window.deleteWAFRule = async function(ip) {
  try {
    const res = await fetch(`/api/dashboard/waf?ip=${encodeURIComponent(ip)}`, {
      method: 'DELETE'
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🗑️ Removed WAF rule for ${ip}`);
    await fetchWAFRules();
  } catch (err) {
    showToast(`❌ Error removing WAF rule: ${err.message}`);
  }
};

// -------------------------------------------------------------
// 13. Circuit Breakers Controller
// -------------------------------------------------------------
async function fetchCircuits() {
  try {
    const res = await fetch('/api/dashboard/circuits');
    if (!res.ok) return;
    state.circuits = await res.json() || [];
    renderCircuits();
  } catch (err) {
    console.warn('Failed to fetch circuits:', err);
  }
}

function renderCircuits() {
  if (!elCircuitsList) return;
  if (state.circuits.length === 0) {
    elCircuitsList.innerHTML = `<div class="empty-cell" style="padding:16px;text-align:center;">No active circuit breakers registered.</div>`;
    return;
  }

  elCircuitsList.innerHTML = state.circuits.map(cb => {
    let stateClass = 'state-closed';
    let dotClass = 'pulse-dot green';
    if (cb.state === 'OPEN') {
      stateClass = 'state-open';
      dotClass = 'pulse-dot' /* red */;
    } else if (cb.state === 'HALF-OPEN') {
      stateClass = 'state-half-open';
      dotClass = 'pulse-dot yellow';
    }

    return `
      <div class="circuit-card">
        <div class="circuit-info">
          <span class="circuit-target">${cb.upstream}</span>
          <span class="circuit-meta">Failures: <strong class="mono" style="color:#ffffff;">${cb.consecutive_failures || cb.failure_count || 0}</strong></span>
        </div>
        <div class="circuit-actions">
          <span class="circuit-state-badge ${stateClass}">
            <span class="${dotClass}"></span>
            ${cb.state}
          </span>
          <button class="btn btn-tiny btn-ghost" title="Reset Circuit to CLOSED" onclick="resetCircuit('${cb.upstream}')">
            🔄 Reset
          </button>
          <button class="btn btn-tiny btn-warning" title="Manually Trip Circuit to OPEN for testing" onclick="tripCircuit('${cb.upstream}')">
            ⚡ Trip
          </button>
        </div>
      </div>
    `;
  }).join('');
}

window.resetCircuit = async function(upstream) {
  try {
    const res = await fetch(`/api/dashboard/circuits/reset?upstream=${encodeURIComponent(upstream)}`, {
      method: 'POST'
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🟢 Circuit Breaker for ${upstream} reset to CLOSED!`);
    await fetchCircuits();
  } catch (err) {
    showToast(`❌ Error resetting circuit: ${err.message}`);
  }
};

window.tripCircuit = async function(upstream) {
  try {
    const res = await fetch(`/api/dashboard/circuits/trip?upstream=${encodeURIComponent(upstream)}`, {
      method: 'POST'
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🚨 Circuit Breaker for ${upstream} tripped to OPEN (Fast-Fail 503)!`);
    await fetchCircuits();
  } catch (err) {
    showToast(`❌ Error tripping circuit: ${err.message}`);
  }
};

// -------------------------------------------------------------
// 14. Dynamic Route Manager
// -------------------------------------------------------------
function openRouteModal() {
  elRouteModal.classList.add('open');
}

function closeRouteModal() {
  elRouteModal.classList.remove('open');
}

async function submitNewRoute() {
  const path = elNewRoutePath.value.trim();
  const upstream = elNewRouteUpstream.value.trim();
  const limit = parseInt(elNewRouteLimit.value, 10) || 100;
  const windowSec = parseInt(elNewRouteWindow.value, 10) || 60;
  const keyType = elNewRouteKeytype.value;
  const requireAuth = elNewRouteAuth.checked;

  if (!path.startsWith('/')) {
    showToast('⚠️ Path prefix must start with "/" (e.g. /api/payments)');
    return;
  }
  if (!upstream.startsWith('http://') && !upstream.startsWith('https://')) {
    showToast('⚠️ Upstream URL must start with http:// or https://');
    return;
  }

  const newRoute = {
    path,
    upstream,
    rate_limit: limit,
    window_sec: windowSec,
    key_type: keyType,
    methods: ['GET', 'POST', 'PUT', 'DELETE'],
    require_auth: requireAuth
  };

  try {
    elBtnSubmitNewRoute.disabled = true;
    elBtnSubmitNewRoute.textContent = 'Saving...';

    const res = await fetch('/api/dashboard/routes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newRoute)
    });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || `HTTP ${res.status}`);
    }

    showToast(`✅ Route "${path}" hot-reloaded successfully!`);
    closeRouteModal();
    elNewRoutePath.value = '';
    elNewRouteUpstream.value = '';
    
    await fetchStats();
    await fetchCircuits();
    await fetchUpstreamNodes();
  } catch (err) {
    showToast(`❌ Error adding route: ${err.message}`);
  } finally {
    elBtnSubmitNewRoute.disabled = false;
    elBtnSubmitNewRoute.textContent = 'Save & Hot-Reload';
  }
}

window.deleteRoute = async function(path) {
  if (!confirm(`Are you sure you want to remove dynamic route "${path}"?`)) return;
  try {
    const res = await fetch(`/api/dashboard/routes?path=${encodeURIComponent(path)}`, {
      method: 'DELETE'
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🗑️ Removed route "${path}"`);
    await fetchStats();
  } catch (err) {
    showToast(`❌ Error deleting route: ${err.message}`);
  }
};

function renderRoutes(routes) {
  if (!elRouteCardsList) return;
  elRouteCardsList.innerHTML = routes.map(r => `
    <div class="route-policy-card">
      <div class="rpc-top">
        <span class="rpc-path">${r.path}</span>
        <div style="display:flex;gap:6px;align-items:center;">
          <span class="rpc-auth-tag ${r.require_auth ? 'required' : 'public'}">
            ${r.require_auth ? '🔒 JWT REQUIRED' : 'PUBLIC'}
          </span>
          <button class="btn btn-tiny btn-ghost" style="color:var(--accent-red);padding:2px 6px;font-size:0.65rem;" title="Remove Route" onclick="deleteRoute('${r.path}')">
            ✖
          </button>
        </div>
      </div>
      <div class="rpc-meta">
        <span>Upstream: <strong>${r.upstream}</strong></span>
        <span>Limit: <strong>${r.rate_limit} req / ${r.window_sec}s</strong></span>
        <span>Key By: <strong>${r.key_type.toUpperCase()}</strong></span>
        <span>Methods: <strong>${(r.methods || []).join(', ')}</strong></span>
      </div>
    </div>
  `).join('');
}

function renderRouteTabs(routes) {
  if (!elRouteTabs) return;
  const currentPath = state.selectedRoute.path;

  elRouteTabs.innerHTML = routes.map((r, i) => {
    const active = (r.path === currentPath) || (!currentPath && i === 0);
    const authTag = r.require_auth ? '🔒 JWT Auth' : (r.key_type === 'ip' ? 'IP Limiter' : 'Public');
    return `
      <button class="tab-btn ${active ? 'active' : ''}" data-path="${r.path}" data-method="GET" data-auth="${r.require_auth}">
        <span class="method-tag get">GET</span>
        <span>${r.path}</span>
        <span class="mini-pill ${r.require_auth ? 'lock' : ''}">${authTag} (${r.rate_limit}/${r.window_sec}s)</span>
      </button>
    `;
  }).join('');
}

function renderBuckets(buckets) {
  if (!elBucketsTbody) return;
  if (!buckets || buckets.length === 0) {
    elBucketsTbody.innerHTML = `<tr><td colspan="4" class="empty-cell">No active rate limit buckets. Send requests to create them!</td></tr>`;
    return;
  }

  elBucketsTbody.innerHTML = buckets.map(b => `
    <tr>
      <td class="bucket-key-cell" title="${b.key}">${b.key}</td>
      <td><strong>${b.count}</strong></td>
      <td>${b.ttl_seconds}s</td>
      <td><span class="tag-pill" style="font-size:0.65rem;">ACTIVE</span></td>
    </tr>
  `).join('');
}

// -------------------------------------------------------------
// 15. Interactive Tester & Request Execution
// -------------------------------------------------------------
async function mintToken() {
  const clientId = elClientId ? elClientId.value.trim() : 'client-vip-42';
  try {
    const res = await fetch('/api/dashboard/token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ client_id: clientId })
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    state.lastToken = data.token;
    if (elJwtToken) elJwtToken.value = data.token;
    showToast(`✅ Minted JWT token for ${clientId}`);
  } catch (err) {
    showToast(`❌ Error minting token: ${err.message}`);
  }
}

async function sendRequest(customIP, customID) {
  animatePipeline();

  const path = elTargetUrl ? elTargetUrl.value : '/api/orders';
  const ip = customIP || (elClientIp ? elClientIp.value.trim() : '127.0.0.1');
  const token = elJwtToken ? elJwtToken.value.trim() : '';
  const apiKey = elApiKey ? elApiKey.value.trim() : '';

  const headers = {
    'X-Forwarded-For': ip,
  };
  if (customID) {
    headers['X-Client-Id'] = customID;
  }
  if (state.selectedRoute.requireAuth && token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  if (apiKey) {
    headers['X-API-Key'] = apiKey;
  }

  const startTime = performance.now();
  try {
    const res = await fetch(path, {
      method: 'GET',
      headers: headers
    });
    const duration = Math.round(performance.now() - startTime);

    const limit = res.headers.get('X-RateLimit-Limit');
    const remaining = res.headers.get('X-RateLimit-Remaining');
    const retryAfter = res.headers.get('Retry-After');
    const cacheHeader = res.headers.get('X-Cache');
    const correlationId = res.headers.get('X-Correlation-ID');
    
    let bodyText = '';
    const contentType = res.headers.get('Content-Type') || '';
    if (contentType.includes('application/json')) {
      const json = await res.json();
      bodyText = JSON.stringify(json, null, 2);
    } else {
      bodyText = await res.text();
    }

    const headerLines = [];
    res.headers.forEach((val, key) => {
      headerLines.push(`${key}: ${val}`);
    });

    return {
      status: res.status,
      duration,
      limit: limit ? parseInt(limit, 10) : null,
      remaining: remaining ? parseInt(remaining, 10) : null,
      retryAfter,
      cacheHeader,
      correlationId,
      bodyText,
      headersText: headerLines.join('\n')
    };
  } catch (err) {
    return {
      status: 0,
      duration: Math.round(performance.now() - startTime),
      bodyText: `Network Error: ${err.message}`,
      headersText: ''
    };
  }
}

function displayResponse(result) {
  if (elResTime) elResTime.textContent = `${result.duration} ms`;

  if (elResBadge) {
    elResBadge.className = 'status-badge';
    if (result.status === 200) {
      elResBadge.classList.add('status-200');
      elResBadge.textContent = result.cacheHeader === 'HIT' ? '200 OK (⚡ CACHE HIT)' : '200 OK';
    } else if (result.status === 429) {
      elResBadge.classList.add('status-429');
      elResBadge.textContent = `429 THROTTLED (Retry: ${result.retryAfter || 60}s)`;
    } else if (result.status === 401) {
      elResBadge.classList.add('status-401');
      elResBadge.textContent = '401 UNAUTHORIZED';
    } else if (result.status === 403) {
      elResBadge.classList.add('status-401');
      elResBadge.textContent = '403 FORBIDDEN (WAF/GeoIP)';
    } else if (result.status === 413) {
      elResBadge.classList.add('status-401');
      elResBadge.textContent = '413 PAYLOAD TOO LARGE';
    } else if (result.status === 503) {
      elResBadge.classList.add('status-401');
      elResBadge.textContent = '503 SERVICE UNAVAILABLE';
    } else if (result.status === 500) {
      elResBadge.classList.add('status-401');
      elResBadge.textContent = '500 INTERNAL SERVER ERROR (CHAOS)';
    } else {
      elResBadge.classList.add('status-idle');
      elResBadge.textContent = `STATUS ${result.status}`;
    }
  }

  if (result.status === 429) showToast(`🚨 Rate Limit Exceeded! 429 returned. Retry after ${result.retryAfter || 60}s`);
  else if (result.status === 401) showToast(`🔒 401 Unauthorized - Valid Bearer Token or API Key required`);
  else if (result.status === 403) showToast(`🛡️ 403 Forbidden - Blocked by WAF or GeoIP Firewall`);
  else if (result.status === 413) showToast(`⚠️ 413 Payload Too Large - Exceeded 1MB body limit`);
  else if (result.status === 503) showToast(`⚡ 503 Service Unavailable - Circuit Breaker Tripped!`);
  else if (result.status === 500) showToast(`💥 500 Internal Server Error (Injected by Chaos Controller)`);

  if (result.limit !== null && result.remaining !== null) {
    if (elLimitTotal) elLimitTotal.textContent = result.limit;
    if (elLimitRemaining) elLimitRemaining.textContent = result.remaining;
    if (elMeterFill) {
      const pct = Math.max(0, Math.min(100, (result.remaining / result.limit) * 100));
      elMeterFill.style.width = `${pct}%`;
      
      if (pct < 15) {
        elMeterFill.className = 'meter-bar-fill empty';
      } else if (pct < 40) {
        elMeterFill.className = 'meter-bar-fill warning';
      } else {
        elMeterFill.className = 'meter-bar-fill';
      }
    }
  }

  if (elResBodyCode) elResBodyCode.textContent = result.bodyText || '// Empty response';
  if (elResHeadersCode) elResHeadersCode.textContent = result.headersText || '// No headers';

  fetchStats();
}

async function runBurst(count, delayMs = 20) {
  showToast(`⚡ Sending burst of ${count} requests...`);
  elBtnBurst10.disabled = true;
  elBtnBurst55.disabled = true;

  let lastResult = null;
  for (let i = 1; i <= count; i++) {
    lastResult = await sendRequest();
    displayResponse(lastResult);
    if (lastResult.status === 429) {
      break;
    }
    if (delayMs > 0) {
      await new Promise(r => setTimeout(r, delayMs));
    }
  }

  elBtnBurst10.disabled = false;
  elBtnBurst55.disabled = false;
}

async function runQuickBenchmark() {
  showToast('🚀 Running 1,000 requests load test...');
  elBtnBenchmark.disabled = true;
  elBtnBenchmark.textContent = 'Running...';

  const promises = [];
  const start = performance.now();
  for (let i = 0; i < 1000; i++) {
    const ip = `10.10.${i % 250}.${Math.floor(i / 250)}`;
    promises.push(sendRequest(ip, `client-${i % 100}`));
  }

  const results = await Promise.all(promises);
  const totalTime = Math.round(performance.now() - start);
  const success = results.filter(r => r.status === 200).length;
  const rateLimited = results.filter(r => r.status === 429).length;

  elBtnBenchmark.disabled = false;
  elBtnBenchmark.textContent = '🚀 Run 1k Load Test';
  showToast(`🏁 1,000 requests finished in ${totalTime}ms! (${success} OK, ${rateLimited} throttled)`);
  
  if (results.length > 0) {
    displayResponse(results[results.length - 1]);
  }
}

// -------------------------------------------------------------
// 16. Event Wiring & Initializers
// -------------------------------------------------------------
function setupEvents() {
  if (elRouteTabs) {
    elRouteTabs.addEventListener('click', e => {
      const btn = e.target.closest('.tab-btn');
      if (!btn) return;

      elRouteTabs.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');

      const path = btn.dataset.path;
      const isAuth = btn.dataset.auth === 'true';

      state.selectedRoute.path = path;
      state.selectedRoute.requireAuth = isAuth;
      if (elTargetUrl) elTargetUrl.value = path;

      if (isAuth && elJwtContainer) {
        elJwtContainer.style.opacity = '1';
        if (elJwtToken && !elJwtToken.value) {
          mintToken();
        }
      }
    });
  }

  document.querySelectorAll('.res-tab').forEach(tab => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.res-tab').forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.code-box').forEach(b => b.classList.remove('active'));

      tab.classList.add('active');
      const targetId = tab.dataset.target;
      const targetEl = document.getElementById(targetId);
      if (targetEl) targetEl.classList.add('active');
    });
  });

  if (elLogFilterPills) {
    elLogFilterPills.addEventListener('click', e => {
      const pill = e.target.closest('.pill');
      if (!pill) return;
      elLogFilterPills.querySelectorAll('.pill').forEach(p => p.classList.remove('active'));
      pill.classList.add('active');
      state.trafficFilter = pill.dataset.filter || 'ALL';
      renderTrafficLogs();
    });
  }

  if (elBtnExportCSV) elBtnExportCSV.addEventListener('click', exportTrafficCSV);
  if (elBtnExportJSON) elBtnExportJSON.addEventListener('click', exportTrafficJSON);

  if (elBtnMint) elBtnMint.addEventListener('click', mintToken);
  if (elBtnTierFree) elBtnTierFree.addEventListener('click', () => { if (elApiKey) elApiKey.value = 'agy_live_free_demo_key'; showToast('Selected Free Tier API Key (10 req/min)'); });
  if (elBtnTierPro) elBtnTierPro.addEventListener('click', () => { if (elApiKey) elApiKey.value = 'agy_live_pro_demo_key'; showToast('Selected Pro Tier API Key (250 req/min)'); });
  if (elBtnTierVip) elBtnTierVip.addEventListener('click', () => { if (elApiKey) elApiKey.value = 'agy_live_enterprise_vip'; showToast('Selected Enterprise VIP API Key (2,500 req/min)'); });

  // Chaos controls
  if (elSliderChaosLatency) {
    elSliderChaosLatency.addEventListener('input', e => {
      if (elValChaosLatency) elValChaosLatency.textContent = `${e.target.value} ms`;
    });
  }
  if (elFaultBtnGroup) {
    elFaultBtnGroup.addEventListener('click', e => {
      const btn = e.target.closest('.pill-fault');
      if (!btn) return;
      elFaultBtnGroup.querySelectorAll('.pill-fault').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      if (elValChaosFault) elValChaosFault.textContent = `${btn.dataset.fault}%`;
    });
  }
  if (elBtnApplyChaos) elBtnApplyChaos.addEventListener('click', applyChaosSettings);
  if (elBtnResetChaos) elBtnResetChaos.addEventListener('click', resetChaosSettings);

  // GeoIP controls
  if (elBtnAddGeoipRule) elBtnAddGeoipRule.addEventListener('click', addGeoIPRule);

  if (elBtnSendSingle) {
    elBtnSendSingle.addEventListener('click', async () => {
      elBtnSendSingle.disabled = true;
      const result = await sendRequest();
      displayResponse(result);
      elBtnSendSingle.disabled = false;
    });
  }

  if (elBtnBurst10) elBtnBurst10.addEventListener('click', () => runBurst(10, 30));
  if (elBtnBurst55) elBtnBurst55.addEventListener('click', () => runBurst(55, 10));
  if (elBtnBenchmark) elBtnBenchmark.addEventListener('click', runQuickBenchmark);

  if (elBtnFlushBuckets) elBtnFlushBuckets.addEventListener('click', flushRedisBuckets);
  if (elBtnAddWafRule) elBtnAddWafRule.addEventListener('click', addWAFRule);
  if (elBtnGenerateApikey) elBtnGenerateApikey.addEventListener('click', generateAPIKey);

  if (elBtnOpenRouteModal) elBtnOpenRouteModal.addEventListener('click', openRouteModal);
  if (elBtnCloseRouteModal) elBtnCloseRouteModal.addEventListener('click', closeRouteModal);
  if (elBtnCancelRouteModal) elBtnCancelRouteModal.addEventListener('click', closeRouteModal);
  if (elBtnSubmitNewRoute) elBtnSubmitNewRoute.addEventListener('click', submitNewRoute);

  if (elBtnCloseTraceModal) elBtnCloseTraceModal.addEventListener('click', closeTraceModal);
  if (elBtnDismissTraceModal) elBtnDismissTraceModal.addEventListener('click', closeTraceModal);

  window.addEventListener('click', e => {
    if (e.target === elRouteModal) closeRouteModal();
    if (e.target === elTraceModal) closeTraceModal();
  });

  const refreshBtn = document.getElementById('refresh-stats-btn');
  if (refreshBtn) {
    refreshBtn.addEventListener('click', () => {
      fetchStats();
      fetchTrafficLogs();
      fetchWAFRules();
      fetchCircuits();
      fetchUpstreamNodes();
      fetchAPIKeys();
      fetchChaosConfig();
      fetchGeoIPRules();
      fetchCloudStatus();
      showToast('🔄 Synchronized metrics and system state');
    });
  }

  const refreshBucketsBtn = document.getElementById('btn-refresh-buckets');
  if (refreshBucketsBtn) refreshBucketsBtn.addEventListener('click', fetchStats);

  // Cloud Hub event listeners
  const btnAddWebhook = document.getElementById('btn-add-webhook');
  if (btnAddWebhook) btnAddWebhook.addEventListener('click', addCloudWebhook);

  const btnTestWebhook = document.getElementById('btn-test-cloud-webhook');
  if (btnTestWebhook) btnTestWebhook.addEventListener('click', testCloudWebhook);

  const btnSyncS3 = document.getElementById('btn-sync-s3-archive');
  if (btnSyncS3) btnSyncS3.addEventListener('click', syncS3Archive);

  const elSpecsTabs = document.getElementById('specs-tabs');
  if (elSpecsTabs) {
    elSpecsTabs.addEventListener('click', e => {
      const btn = e.target.closest('.spec-tab-btn');
      if (!btn) return;
      elSpecsTabs.querySelectorAll('.spec-tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.spec-content').forEach(c => c.classList.remove('active'));

      btn.classList.add('active');
      const targetId = btn.dataset.tab;
      const targetEl = document.getElementById(targetId);
      if (targetEl) targetEl.classList.add('active');
    });
  }

  const elBtnOpenSpecs = document.getElementById('btn-open-specs');
  if (elBtnOpenSpecs) {
    elBtnOpenSpecs.addEventListener('click', () => {
      const panel = document.getElementById('specs-panel');
      if (panel) panel.scrollIntoView({ behavior: 'smooth' });
    });
  }
}

// -------------------------------------------------------------
// 17. Enterprise Cloud Integration Hub
// -------------------------------------------------------------
async function fetchCloudStatus() {
  const regionsGrid = document.getElementById('regions-grid');
  const webhooksTbody = document.getElementById('webhooks-tbody');
  const archivesTbody = document.getElementById('archives-tbody');

  if (!regionsGrid && !webhooksTbody && !archivesTbody) return;

  try {
    const res = await fetch('/api/dashboard/cloud/status');
    if (!res.ok) return;
    const data = await res.json();

    // 1. Render Regions
    if (regionsGrid && data.regions) {
      regionsGrid.innerHTML = data.regions.map(r => {
        const flagMap = {
          'us-east-1': '🇺🇸',
          'eu-west-1': '🇪🇺',
          'ap-south-1': '🇮🇳',
        };
        const flag = flagMap[r.code] || '🌐';
        const isMaster = r.role === 'primary';
        return `
          <div class="region-card">
            <div class="region-header">
              <div class="region-title">
                <span>${flag}</span>
                <span>${r.name}</span>
                ${isMaster ? '<span class="tag-pill" style="font-size:0.6rem;padding:1px 6px;">PRIMARY</span>' : ''}
              </div>
              <span class="region-ping">${r.latency_ms} ms</span>
            </div>
            <div class="region-meta">
              <div class="region-meta-row">
                <span>Provider / Region:</span>
                <code class="mono text-cyan">AWS (${r.code})</code>
              </div>
              <div class="region-meta-row">
                <span>Replication:</span>
                <strong class="mono text-green">${r.replication}</strong>
              </div>
              <div class="region-meta-row">
                <span>Sync Status:</span>
                <span class="node-status-badge node-healthy"><span class="pulse-dot green"></span>${(r.status || 'healthy').toUpperCase()}</span>
              </div>
            </div>
          </div>
        `;
      }).join('');
    }

    // 2. Render Webhooks
    if (webhooksTbody && data.webhooks) {
      if (data.webhooks.length === 0) {
        webhooksTbody.innerHTML = `<tr><td colspan="5" class="empty-cell">No cloud webhooks registered yet.</td></tr>`;
      } else {
        webhooksTbody.innerHTML = data.webhooks.map(w => {
          const eventsList = (w.events || []).join(', ');
          const secretPreview = w.secret ? w.secret.substring(0, 10) + '...' : 'none';
          return `
            <tr>
              <td class="mono" style="font-weight:600;color:#ffffff;">${w.name}</td>
              <td><span class="tag-pill">${(w.provider || 'generic').toUpperCase()}</span></td>
              <td class="mono" style="font-size:0.75rem;color:var(--accent-mint);">${eventsList}</td>
              <td class="mono" style="font-size:0.72rem;color:var(--text-muted);" title="${w.secret}">${secretPreview}</td>
              <td>
                <button class="btn btn-tiny btn-ghost" style="color:var(--accent-red);padding:2px 6px;" onclick="deleteCloudWebhook('${w.id}')">✖ Delete</button>
              </td>
            </tr>
          `;
        }).join('');
      }
    }

    // 3. Render Archives
    if (archivesTbody && data.archives) {
      if (data.archives.length === 0) {
        archivesTbody.innerHTML = `<tr><td colspan="5" class="empty-cell">No archives generated yet. Click "Sync to S3 Bucket" to archive logs.</td></tr>`;
      } else {
        archivesTbody.innerHTML = data.archives.map(a => {
          const s3Uri = a.s3_uri || `s3://api-gateway-traffic-logs/${a.id}.json.gz`;
          const sizeKb = (a.size_bytes / 1024).toFixed(1);
          return `
            <tr>
              <td class="mono" style="font-size:0.72rem;color:var(--text-muted);">${a.id}</td>
              <td class="mono text-green" style="font-size:0.78rem;">${s3Uri}</td>
              <td class="mono">${formatNum(a.record_count)}</td>
              <td class="mono text-cyan">${sizeKb} KB</td>
              <td><span class="node-status-badge node-healthy"><span class="pulse-dot green"></span>SYNCED</span></td>
            </tr>
          `;
        }).join('');
      }
    }
  } catch (err) {
    console.warn('Failed to fetch cloud status:', err);
  }
}

async function addCloudWebhook() {
  const nameInput = document.getElementById('input-webhook-name');
  const urlInput = document.getElementById('input-webhook-url');
  const providerSelect = document.getElementById('select-webhook-provider');

  if (!nameInput || !urlInput || !providerSelect) return;

  const name = nameInput.value.trim();
  const url = urlInput.value.trim();
  const provider = providerSelect.value;

  if (!name || !url) {
    showToast('⚠️ Please enter a Webhook Name and valid URL');
    return;
  }

  try {
    const res = await fetch('/api/dashboard/cloud/webhooks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        url,
        provider,
        events: ['rate_limit_spike', 'circuit_trip', 'waf_block', 'geoip_block']
      })
    });

    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`☁️ Cloud Webhook "${name}" registered with HMAC-SHA256 signature!`);
    nameInput.value = '';
    urlInput.value = '';
    fetchCloudStatus();
  } catch (err) {
    showToast(`❌ Failed to add webhook: ${err.message}`);
  }
}

window.deleteCloudWebhook = async function(id) {
  try {
    const res = await fetch(`/api/dashboard/cloud/webhooks?id=${encodeURIComponent(id)}`, {
      method: 'DELETE'
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🗑️ Webhook deleted successfully`);
    fetchCloudStatus();
  } catch (err) {
    showToast(`❌ Error deleting webhook: ${err.message}`);
  }
};

async function testCloudWebhook() {
  const btn = document.getElementById('btn-test-cloud-webhook');
  if (btn) btn.disabled = true;

  try {
    const res = await fetch('/api/dashboard/cloud/test-webhook', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        event: 'rate_limit_spike',
        message: 'High traffic anomaly detected on /api/orders (950 req/min)',
        severity: 'WARNING'
      })
    });
    const data = await res.json();
    showToast(`🚀 Test Alert Dispatched to ${data.dispatched || 0} cloud webhook endpoints!`);
  } catch (err) {
    showToast(`❌ Webhook test failed: ${err.message}`);
  } finally {
    if (btn) btn.disabled = false;
  }
}

async function syncS3Archive() {
  const btn = document.getElementById('btn-sync-s3-archive');
  if (btn) {
    btn.disabled = true;
    btn.textContent = '⏳ Archiving...';
  }

  try {
    const res = await fetch('/api/dashboard/cloud/archive', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });
    const data = await res.json();
    showToast(`☁️ Archived ${data.record_count} logs to s3://${data.bucket}/${data.file_name} (${data.compression})`);
    fetchCloudStatus();
  } catch (err) {
    showToast(`❌ Archival failed: ${err.message}`);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = '☁️ Sync to S3 Bucket';
    }
  }
}

// Initialize on Load
window.addEventListener('DOMContentLoaded', () => {
  setupEvents();
  setupMobileAppEvents();
  initSSE();
  fetchStats();
  fetchTrafficLogs();
  fetchWAFRules();
  fetchCircuits();
  fetchUpstreamNodes();
  fetchAPIKeys();
  fetchChaosConfig();
  fetchGeoIPRules();
  fetchCloudStatus();

  setInterval(fetchStats, 2000);
  setInterval(fetchCircuits, 3000);
  setInterval(fetchUpstreamNodes, 3000);
  setInterval(fetchCloudStatus, 4000);
});

// =========================================================
// PWA & Mobile App Experience Controller
// =========================================================

// 1. Register Service Worker for PWA
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js')
      .then((reg) => console.log('PWA ServiceWorker registered:', reg.scope))
      .catch((err) => console.log('PWA ServiceWorker failed:', err));
  });
}

// 2. Mobile PWA Install Prompt Banner
let deferredPrompt = null;
window.addEventListener('beforeinstallprompt', (e) => {
  e.preventDefault();
  deferredPrompt = e;
  
  // Show install banner if not dismissed before
  if (!sessionStorage.getItem('pwa-dismissed')) {
    const banner = document.getElementById('pwa-install-banner');
    if (banner) {
      banner.style.display = 'flex';
    }
  }
});

// Setup PWA & Mobile Navigation Events
function setupMobileAppEvents() {
  const installBtn = document.getElementById('btn-pwa-install');
  const closeBtn = document.getElementById('btn-pwa-close');
  const banner = document.getElementById('pwa-install-banner');

  if (installBtn) {
    installBtn.addEventListener('click', async () => {
      if (deferredPrompt) {
        deferredPrompt.prompt();
        const { outcome } = await deferredPrompt.userChoice;
        console.log('PWA Install choice:', outcome);
        deferredPrompt = null;
        if (banner) banner.style.display = 'none';
      } else {
        alert('To install on your phone:\n\n• Android: Tap menu (⋮) -> "Add to Home screen"\n• iOS: Tap Share button -> "Add to Home Screen"');
      }
    });
  }

  if (closeBtn && banner) {
    closeBtn.addEventListener('click', () => {
      banner.style.display = 'none';
      sessionStorage.setItem('pwa-dismissed', 'true');
    });
  }

  // Mobile More Drawer Toggle with Backdrop
  const mobMoreBtn = document.getElementById('mob-nav-more-btn');
  const mobDrawer = document.getElementById('mob-more-drawer');
  const mobDrawerClose = document.getElementById('mob-drawer-close');
  const mobBackdrop = document.getElementById('mob-drawer-backdrop');

  function openMobileDrawer() {
    if (mobDrawer) mobDrawer.classList.add('open');
    if (mobBackdrop) mobBackdrop.classList.add('open');
  }

  function closeMobileDrawer() {
    if (mobDrawer) mobDrawer.classList.remove('open');
    if (mobBackdrop) mobBackdrop.classList.remove('open');
  }

  if (mobMoreBtn) {
    mobMoreBtn.addEventListener('click', (e) => {
      e.preventDefault();
      if (mobDrawer && mobDrawer.classList.contains('open')) {
        closeMobileDrawer();
      } else {
        openMobileDrawer();
      }
    });
  }

  if (mobDrawerClose) mobDrawerClose.addEventListener('click', closeMobileDrawer);
  if (mobBackdrop) mobBackdrop.addEventListener('click', closeMobileDrawer);

  // Keyboard shortcut: Ctrl+K / Cmd+K for AI Copilot, Escape for all drawers
  window.addEventListener('keydown', (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      const aiDrawer = document.getElementById('ai-copilot-drawer');
      if (aiDrawer) aiDrawer.classList.toggle('open');
    }
    if (e.key === 'Escape') {
      closeMobileDrawer();
      const aiDrawer = document.getElementById('ai-copilot-drawer');
      if (aiDrawer) aiDrawer.classList.remove('open');
    }
  });

  // Call New Real-Time & AI Initializers
  initFloatingAICopilot();
  initAICopilotHub();
  initThreatRadar();
  initRealTimeSimulatorControls();
  initCanaryRouterControls();
  fetchAutoBannedIPs();
}

// Dynamic Animated Number Helper
function animateNumber(el, targetVal, duration = 350) {
  if (!el) return;
  const startVal = parseInt(el.textContent.replace(/[^0-9]/g, '')) || 0;
  if (startVal === targetVal) return;
  const startTime = performance.now();

  function step(now) {
    const progress = Math.min(1, (now - startTime) / duration);
    const current = Math.round(startVal + (targetVal - startVal) * progress);
    el.textContent = formatNum(current);
    if (progress < 1) {
      requestAnimationFrame(step);
    } else {
      el.textContent = formatNum(targetVal);
    }
  }
  requestAnimationFrame(step);
}

// =========================================================
// Real-Time SSE Stream Controller (Sub-Second Telemetry)
// =========================================================
function initSSE() {
  if (!window.EventSource) return;
  const evtSource = new EventSource('/api/dashboard/stream');

  evtSource.addEventListener('connected', () => {
    if (elSseBadge) elSseBadge.innerHTML = '<span class="pulse-dot green"></span><span>SSE LIVE</span>';
  });

  evtSource.addEventListener('stats', (e) => {
    try {
      const d = JSON.parse(e.data);
      if (elRps && d.rps !== undefined) elRps.textContent = `+${Math.round(d.rps)} req/s`;
      if (elTotalReqs && d.total_requests !== undefined) animateNumber(elTotalReqs, d.total_requests);
      if (elRateLimited && d.total_rate_limited !== undefined) animateNumber(elRateLimited, d.total_rate_limited);

      // Flash card pulse on live request
      if (d.rps > 0) {
        document.querySelectorAll('.glow-card').forEach(c => {
          c.classList.remove('pulse-glow-card');
          void c.offsetWidth;
          c.classList.add('pulse-glow-card');
        });
      }

      const clientEl = document.getElementById('rt-connected-clients');
      if (clientEl && d.connected_clients !== undefined) {
        clientEl.textContent = `🟢 ${d.connected_clients} Device${d.connected_clients === 1 ? '' : 's'} Online`;
      }

      if (d.simulator) {
        const statusBadge = document.getElementById('rt-engine-status');
        const toggleBtn = document.getElementById('btn-toggle-simulator');
        if (statusBadge) {
          statusBadge.textContent = d.simulator.active ? 'ACTIVE' : 'PAUSED';
          statusBadge.className = d.simulator.active ? 'rt-badge text-green' : 'rt-badge text-yellow';
        }
        if (toggleBtn) {
          toggleBtn.textContent = d.simulator.active ? '⏸️ Pause Stream' : '▶️ Resume Stream';
        }
      }

      if (state.historyRPS) {
        state.historyRPS.shift();
        state.historyRPS.push(d.rps || 0);
        drawSparkline();
      }

      updateTokenBucket(d.rps || 12);
    } catch (err) {}
  });

  evtSource.addEventListener('threat', (e) => {
    try {
      const ping = JSON.parse(e.data);
      if (window.addRadarThreatPing) {
        window.addRadarThreatPing(ping);
      }
    } catch (err) {}
  });

  evtSource.addEventListener('traffic', (e) => {
    try {
      const log = JSON.parse(e.data);
      state.trafficLogs.unshift(log);
      if (state.trafficLogs.length > 50) state.trafficLogs.pop();
      renderTrafficLogs();
    } catch (err) {}
  });

  evtSource.addEventListener('alert', (e) => {
    try {
      const alertData = JSON.parse(e.data);
      showToast(`🚨 AI Security Alert: ${alertData.reason || 'Anomaly Detected'} on IP ${alertData.ip}`);
      fetchAutoBannedIPs();
    } catch (err) {}
  });

  evtSource.onerror = () => {
    if (elSseBadge) elSseBadge.innerHTML = '<span class="pulse-dot yellow"></span><span>SSE RECONNECTING</span>';
  };
}

// =========================================================
// Real-Time Global Threat & Traffic Radar
// =========================================================
function initThreatRadar() {
  const canvas = document.getElementById('threat-radar-canvas');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');

  function resizeCanvas() {
    if (canvas.offsetWidth > 0) {
      canvas.width = canvas.offsetWidth;
      canvas.height = Math.min(230, Math.max(180, canvas.offsetWidth * 0.42));
    }
  }
  resizeCanvas();
  window.addEventListener('resize', resizeCanvas);

  canvas.style.cursor = 'crosshair';
  canvas.title = 'Click anywhere on radar to ping telemetry coordinates';

  let angle = 0;
  const pings = [];

  const hubs = [
    { name: 'US-East', x: 0.22, y: 0.38, country: 'US' },
    { name: 'EU-West', x: 0.48, y: 0.32, country: 'GB' },
    { name: 'AP-South (Mumbai)', x: 0.68, y: 0.48, country: 'IN' },
    { name: 'AP-Northeast (Tokyo)', x: 0.85, y: 0.36, country: 'JP' },
    { name: 'SA-East (São Paulo)', x: 0.32, y: 0.75, country: 'BR' },
    { name: 'AP-Southeast (SG)', x: 0.76, y: 0.58, country: 'SG' },
  ];

  window.addRadarThreatPing = function(ping) {
    let targetHub = hubs.find(h => h.country === ping.country) || hubs[0];
    pings.push({
      x: targetHub.x * canvas.width,
      y: targetHub.y * canvas.height,
      status: ping.status,
      radius: 4,
      maxRadius: 28,
      alpha: 1.0,
      isThreat: ping.isThreat,
    });
    if (pings.length > 25) pings.shift();
  };

  canvas.addEventListener('click', (e) => {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    pings.push({
      x, y,
      status: 200,
      radius: 3,
      maxRadius: 36,
      alpha: 1.0,
      isThreat: false
    });
    showToast(`🎯 Threat Radar scanned coordinates (${Math.round(x)}, ${Math.round(y)})`);
  });

  function renderRadar() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);

    // 1. Grid Background & Concentric Rings
    const cx = canvas.width / 2;
    const cy = canvas.height / 2;

    ctx.strokeStyle = 'rgba(0, 255, 136, 0.12)';
    ctx.lineWidth = 1;
    for (let r = 35; r < Math.max(canvas.width, canvas.height); r += 35) {
      ctx.beginPath();
      ctx.arc(cx, cy, r, 0, Math.PI * 2);
      ctx.stroke();
    }

    // Crosshairs
    ctx.beginPath();
    ctx.moveTo(0, cy);
    ctx.lineTo(canvas.width, cy);
    ctx.moveTo(cx, 0);
    ctx.lineTo(cx, canvas.height);
    ctx.stroke();

    // 2. Rotating Radar Sweep Line with phosphor decay
    angle += 0.028;
    ctx.save();
    ctx.translate(cx, cy);
    ctx.rotate(angle);
    const grad = ctx.createLinearGradient(0, 0, cx, 0);
    grad.addColorStop(0, 'rgba(0, 255, 136, 0.45)');
    grad.addColorStop(1, 'rgba(0, 255, 136, 0)');
    ctx.fillStyle = grad;
    ctx.beginPath();
    ctx.moveTo(0, 0);
    ctx.arc(0, 0, Math.max(canvas.width, canvas.height), -0.28, 0);
    ctx.closePath();
    ctx.fill();
    ctx.restore();

    // 3. Draw Global Hubs
    hubs.forEach(h => {
      const hx = h.x * canvas.width;
      const hy = h.y * canvas.height;

      ctx.fillStyle = 'rgba(0, 255, 136, 0.85)';
      ctx.beginPath();
      ctx.arc(hx, hy, 4, 0, Math.PI * 2);
      ctx.fill();

      ctx.fillStyle = '#86a894';
      ctx.font = '9px JetBrains Mono';
      ctx.fillText(h.name, hx + 7, hy + 3);
    });

    // 4. Draw Animated Ping Ripples
    for (let i = pings.length - 1; i >= 0; i--) {
      const p = pings[i];
      p.radius += 0.8;
      p.alpha -= 0.022;

      if (p.alpha <= 0) {
        pings.splice(i, 1);
        continue;
      }

      ctx.strokeStyle = p.status === 403 ? `rgba(244, 63, 94, ${p.alpha})` :
                        p.status === 429 ? `rgba(250, 204, 21, ${p.alpha})` :
                                           `rgba(0, 255, 136, ${p.alpha})`;
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2);
      ctx.stroke();
    }

    requestAnimationFrame(renderRadar);
  }

  renderRadar();
}

// =========================================================
// Sliding Window Token Bucket Visualizer
// =========================================================
function updateTokenBucket(rps) {
  const liquidFill = document.getElementById('bucket-liquid-fill');
  const tokensNum = document.getElementById('bucket-tokens-num');
  const leakStat = document.getElementById('b-stat-leak');
  const healthStat = document.getElementById('b-stat-health');
  const bucketVisual = document.querySelector('.bucket-visual');

  if (!liquidFill || !tokensNum) return;

  const currentTokens = Math.max(10, Math.min(100, Math.round(100 - (rps * 1.5))));
  liquidFill.style.height = `${currentTokens}%`;
  tokensNum.textContent = currentTokens;

  if (bucketVisual && !bucketVisual.dataset.wired) {
    bucketVisual.dataset.wired = 'true';
    bucketVisual.style.cursor = 'pointer';
    bucketVisual.title = 'Click to test token consumption';
    bucketVisual.addEventListener('click', () => {
      liquidFill.style.transform = 'scaleY(0.85)';
      setTimeout(() => liquidFill.style.transform = 'scaleY(1)', 200);
      showToast('💧 Token consumed! Sliding-window counter updated in Redis.');
    });
  }

  if (leakStat) leakStat.textContent = `${Math.round(rps)} req/s`;
  if (healthStat) {
    if (currentTokens < 25) {
      healthStat.textContent = 'HIGH LOAD (NEAR 429)';
      healthStat.className = 'mono text-red';
    } else if (currentTokens < 60) {
      healthStat.textContent = 'MODERATE';
      healthStat.className = 'mono text-yellow';
    } else {
      healthStat.textContent = 'NORMAL';
      healthStat.className = 'mono text-green';
    }
  }
}

// =========================================================
// Real-Time Traffic Simulator Controls
// =========================================================
function initRealTimeSimulatorControls() {
  const toggleBtn = document.getElementById('btn-toggle-simulator');
  const rpsSlider = document.getElementById('slider-simulator-rps');
  const rpsVal = document.getElementById('val-sim-rps');
  const burstBtn = document.getElementById('btn-sim-burst');
  const openCopilotBtn = document.getElementById('btn-open-copilot-banner');

  if (toggleBtn) {
    toggleBtn.addEventListener('click', async () => {
      const isPaused = toggleBtn.textContent.includes('Resume');
      const action = isPaused ? 'resume' : 'pause';
      try {
        const res = await fetch('/api/dashboard/simulator', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action })
        });
        const data = await res.json();
        showToast(data.active ? '▶️ Live Real-Time Stream Resumed' : '⏸️ Live Stream Paused');
      } catch (err) {
        showToast(`❌ Error: ${err.message}`);
      }
    });
  }

  if (rpsSlider) {
    rpsSlider.addEventListener('input', async (e) => {
      const rps = parseInt(e.target.value);
      if (rpsVal) rpsVal.textContent = rps;
      try {
        await fetch('/api/dashboard/simulator', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action: 'set_rps', rps })
        });
      } catch (err) {}
    });
  }

  if (burstBtn) {
    burstBtn.addEventListener('click', async () => {
      burstBtn.disabled = true;
      burstBtn.textContent = '🔥 Sending Burst...';
      try {
        await fetch('/api/dashboard/simulator', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action: 'burst', count: 50 })
        });
        showToast('🔥 Injected 50 concurrent requests into Gateway pipeline!');
      } catch (err) {
        showToast(`❌ Burst failed: ${err.message}`);
      } finally {
        setTimeout(() => {
          burstBtn.disabled = false;
          burstBtn.textContent = '🔥 Burst 50 Reqs';
        }, 800);
      }
    });
  }

  if (openCopilotBtn) {
    openCopilotBtn.addEventListener('click', () => {
      const drawer = document.getElementById('ai-copilot-drawer');
      if (drawer) drawer.classList.add('open');
    });
  }
}

// =========================================================
// AI Auto-Ban Controller (security.html)
// =========================================================
async function fetchAutoBannedIPs() {
  const tbody = document.getElementById('autoban-tbody');
  if (!tbody) return;

  try {
    const res = await fetch('/api/dashboard/security/autoban');
    if (!res.ok) return;
    const list = await res.json();

    if (!list || list.length === 0) {
      tbody.innerHTML = `<tr><td colspan="5" class="empty-cell">No active auto-banned IPs. AI Anomaly Detector is monitoring.</td></tr>`;
      return;
    }

    tbody.innerHTML = list.map(item => `
      <tr>
        <td class="mono font-bold text-red">${item.ip}</td>
        <td>${item.reason}</td>
        <td><span class="badge" style="color:var(--accent-red);border-color:var(--accent-red);">${item.risk_score} / 100</span></td>
        <td class="mono text-yellow">${item.ttl_seconds}s remaining</td>
        <td>
          <button class="btn btn-tiny btn-ghost" style="color:var(--primary-green);" onclick="unbanIP('${item.ip}')">🔓 Unban</button>
        </td>
      </tr>
    `).join('');
  } catch (err) {
    console.warn('Failed to fetch auto-ban list:', err);
  }
}

window.unbanIP = async function(ip) {
  try {
    const res = await fetch(`/api/dashboard/security/autoban?ip=${encodeURIComponent(ip)}`, { method: 'POST' });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    showToast(`🔓 IP ${ip} unbanned successfully!`);
    fetchAutoBannedIPs();
  } catch (err) {
    showToast(`❌ Unban failed: ${err.message}`);
  }
};

// =========================================================
// Canary A/B Traffic Splitting Controller (routes.html)
// =========================================================
function initCanaryRouterControls() {
  const slider = document.getElementById('slider-canary-weight');
  const stablePct = document.getElementById('val-canary-stable-pct');
  const canaryPct = document.getElementById('val-canary-canary-pct');
  const totalReqs = document.getElementById('canary-total-reqs');
  const errorRate = document.getElementById('canary-error-rate');

  if (!slider) return;

  async function updateCanary(weight) {
    slider.value = weight;
    if (canaryPct) canaryPct.textContent = `${weight}%`;
    if (stablePct) stablePct.textContent = `${100 - weight}%`;

    try {
      const res = await fetch('/api/dashboard/routes/canary', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ weight })
      });
      const data = await res.json();
      showToast(`🔀 Canary Traffic Weight updated to ${weight}% (v2-canary) / ${100 - weight}% (v1-stable)`);
      if (totalReqs) totalReqs.textContent = formatNum(data.canary_requests);
      if (errorRate) errorRate.textContent = `${data.canary_error_pct.toFixed(1)}%`;
    } catch (err) {
      showToast(`❌ Canary update failed: ${err.message}`);
    }
  }

  slider.addEventListener('change', (e) => {
    updateCanary(parseInt(e.target.value));
  });

  document.querySelectorAll('.canary-preset-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const w = parseInt(btn.dataset.weight);
      updateCanary(w);
    });
  });

  // Initial fetch
  fetch('/api/dashboard/routes/canary')
    .then(r => r.json())
    .then(data => {
      if (slider) slider.value = data.canary_weight;
      if (canaryPct) canaryPct.textContent = `${data.canary_weight}%`;
      if (stablePct) stablePct.textContent = `${data.stable_weight}%`;
      if (totalReqs) totalReqs.textContent = formatNum(data.canary_requests);
      if (errorRate) errorRate.textContent = `${data.canary_error_pct.toFixed(1)}%`;
    }).catch(() => {});
}

// =========================================================
// Floating AI Copilot & Slide-Out Drawer (All Pages)
// =========================================================
function initFloatingAICopilot() {
  // Inject floating button if not present
  if (!document.getElementById('btn-floating-copilot')) {
    const btn = document.createElement('button');
    btn.id = 'btn-floating-copilot';
    btn.className = 'floating-copilot-btn';
    btn.innerHTML = `🤖 AI Copilot <span class="ai-sparkle">✨</span>`;
    document.body.appendChild(btn);
  }

  // Inject slide-out drawer if not present
  if (!document.getElementById('ai-copilot-drawer')) {
    const drawer = document.createElement('div');
    drawer.id = 'ai-copilot-drawer';
    drawer.className = 'ai-copilot-drawer';
    drawer.innerHTML = `
      <div class="ai-drawer-header">
        <div class="ai-drawer-title">🤖 Gateway AI Copilot</div>
        <button id="btn-close-ai-drawer" class="ai-drawer-close">✕</button>
      </div>
      <div class="ai-chat-history" id="drawer-chat-history">
        <div class="ai-msg ai-msg-bot">
          <div class="ai-avatar">🤖</div>
          <div class="ai-bubble">
            <strong>Gateway AI Copilot:</strong> How can I assist with your API Gateway configuration, WAF security, or incident diagnosis today?
            <div class="ai-quick-chips">
              <button class="ai-chip" data-prompt="Run incident diagnostics on gateway health">🩺 Diagnose Health</button>
              <button class="ai-chip" data-prompt="Block IP 198.51.100.77 for malicious crawling">🚫 Block Malicious IP</button>
              <button class="ai-chip" data-prompt="Whitelist IP 10.0.0.1 for internal VIP">✅ Whitelist IP</button>
              <button class="ai-chip" data-prompt="Trip circuit breaker on http://localhost:9001">⚡ Trip Circuit Breaker</button>
            </div>
          </div>
        </div>
      </div>
      <div class="ai-input-bar">
        <input type="text" id="drawer-input-copilot" class="ai-input" placeholder="Type directive: 'Block IP 1.2.3.4', 'Diagnose errors'...">
        <button id="drawer-btn-send" class="btn btn-primary btn-tiny">Send</button>
      </div>
    `;
    document.body.appendChild(drawer);
  }

  const floatBtn = document.getElementById('btn-floating-copilot');
  const drawer = document.getElementById('ai-copilot-drawer');
  const closeBtn = document.getElementById('btn-close-ai-drawer');
  const sendBtn = document.getElementById('drawer-btn-send');
  const inputEl = document.getElementById('drawer-input-copilot');
  const chatHistory = document.getElementById('drawer-chat-history');

  if (floatBtn && drawer) {
    floatBtn.addEventListener('click', () => drawer.classList.add('open'));
  }
  if (closeBtn && drawer) {
    closeBtn.addEventListener('click', () => drawer.classList.remove('open'));
  }

  async function handleSendDirective(prompt) {
    if (!prompt) return;

    // Append User Message
    const userMsg = document.createElement('div');
    userMsg.className = 'ai-msg ai-msg-user';
    userMsg.innerHTML = `<div class="ai-bubble">${prompt}</div>`;
    chatHistory.appendChild(userMsg);
    chatHistory.scrollTop = chatHistory.scrollHeight;

    // Check if diagnostic
    if (prompt.toLowerCase().includes('diagnose')) {
      try {
        const res = await fetch('/api/dashboard/ai/diagnose');
        const diag = await res.json();
        const botMsg = document.createElement('div');
        botMsg.className = 'ai-msg ai-msg-bot';
        botMsg.innerHTML = `
          <div class="ai-avatar">🤖</div>
          <div class="ai-bubble">
            <strong>Incident Diagnostic Report:</strong><br>
            • <strong>Status:</strong> <span class="mono ${diag.health_status === 'OPTIMAL' ? 'text-green' : 'text-yellow'}">${diag.health_status} (${diag.health_score}/100)</span><br>
            • <strong>Active RPS:</strong> <span class="mono">${diag.active_rps.toFixed(1)} req/s</span><br>
            • <strong>Findings:</strong><br>${diag.findings.map(f => `  - ${f}`).join('<br>')}<br>
            • <strong>Recommended Action:</strong><br>${diag.remediations.map(r => `  - ${r}`).join('<br>')}
          </div>
        `;
        chatHistory.appendChild(botMsg);
        chatHistory.scrollTop = chatHistory.scrollHeight;
        return;
      } catch (err) {}
    }

    try {
      const res = await fetch('/api/dashboard/ai/copilot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prompt })
      });
      const data = await res.json();

      const botMsg = document.createElement('div');
      botMsg.className = 'ai-msg ai-msg-bot';
      botMsg.innerHTML = `
        <div class="ai-avatar">🤖</div>
        <div class="ai-bubble">
          <strong>Action: ${data.action}</strong><br>
          ${data.explanation}
        </div>
      `;
      chatHistory.appendChild(botMsg);
      chatHistory.scrollTop = chatHistory.scrollHeight;
      showToast(`🤖 AI Copilot: ${data.action} executed!`);
      fetchWAFRules();
      fetchGeoIPRules();
    } catch (err) {
      showToast(`❌ AI Copilot error: ${err.message}`);
    }
  }

  if (sendBtn && inputEl) {
    sendBtn.addEventListener('click', () => {
      const val = inputEl.value.trim();
      inputEl.value = '';
      handleSendDirective(val);
    });
    inputEl.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const val = inputEl.value.trim();
        inputEl.value = '';
        handleSendDirective(val);
      }
    });
  }

  if (chatHistory) {
    chatHistory.addEventListener('click', (e) => {
      const chip = e.target.closest('.ai-chip');
      if (chip) {
        handleSendDirective(chip.dataset.prompt);
      }
    });
  }
}

// =========================================================
// AI Hub Page Controller (ai.html)
// =========================================================
function initAICopilotHub() {
  const submitPromptBtn = document.getElementById('btn-submit-ai-prompt');
  const promptInput = document.getElementById('input-ai-prompt');
  const modelSelect = document.getElementById('select-ai-model');
  const testResult = document.getElementById('ai-test-result');

  const statusBadge = document.getElementById('ai-res-status-badge');
  const cacheBadge = document.getElementById('ai-res-cache-badge');
  const tokensBadge = document.getElementById('ai-res-tokens-badge');
  const latencyBadge = document.getElementById('ai-res-latency');
  const bodyCode = document.getElementById('ai-res-body-code');

  const copilotInput = document.getElementById('input-ai-copilot');
  const sendCopilotBtn = document.getElementById('btn-send-ai-copilot');
  const copilotChatHistory = document.getElementById('ai-copilot-chat-history');
  const quickDiagBtn = document.getElementById('btn-copilot-quick-diag');

  // Sample prompt buttons
  document.querySelectorAll('.sample-ai-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      if (promptInput) promptInput.value = btn.dataset.prompt;
    });
  });

  if (submitPromptBtn && promptInput) {
    submitPromptBtn.addEventListener('click', async () => {
      const prompt = promptInput.value.trim();
      if (!prompt) {
        showToast('⚠️ Please enter a prompt');
        return;
      }

      submitPromptBtn.disabled = true;
      submitPromptBtn.textContent = 'Evaluating...';

      try {
        const start = performance.now();
        const res = await fetch('/api/ai/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            prompt,
            model: modelSelect ? modelSelect.value : 'gemini-1.5-flash'
          })
        });
        const duration = Math.round(performance.now() - start);
        const data = await res.json();

        if (testResult) testResult.style.display = 'block';

        if (res.status === 403) {
          if (statusBadge) {
            statusBadge.className = 'badge text-red';
            statusBadge.textContent = '403 BLOCKED';
          }
          if (cacheBadge) cacheBadge.textContent = 'AI_SECURITY_BLOCK';
          if (tokensBadge) tokensBadge.textContent = '0 Tokens';
          if (latencyBadge) latencyBadge.textContent = `${duration} ms`;
          if (bodyCode) bodyCode.textContent = JSON.stringify(data, null, 2);
          showToast(`🛡️ AI Prompt Injection Guard Blocked Adversarial Attack!`);
        } else {
          if (statusBadge) {
            statusBadge.className = 'badge text-green';
            statusBadge.textContent = '200 OK';
          }
          const cacheHeader = res.headers.get('X-Cache') || (data.cached ? 'AI_SEMANTIC_HIT' : 'AI_SEMANTIC_MISS');
          if (cacheBadge) cacheBadge.textContent = cacheHeader;
          if (tokensBadge) tokensBadge.textContent = `${data.prompt_tokens + data.completion_tokens} Tokens`;
          if (latencyBadge) latencyBadge.textContent = `${duration} ms`;
          if (bodyCode) bodyCode.textContent = JSON.stringify(data, null, 2);
          showToast(cacheHeader === 'AI_SEMANTIC_HIT' ? '⚡ Semantic Cache Hit (<1ms)!' : '🚀 AI Prompt Processed');
        }
      } catch (err) {
        showToast(`❌ AI Test failed: ${err.message}`);
      } finally {
        submitPromptBtn.disabled = false;
        submitPromptBtn.textContent = '🚀 Test AI Gateway';
      }
    });
  }

  // Hub Copilot Chat
  async function sendHubCopilot(prompt) {
    if (!prompt || !copilotChatHistory) return;

    const userMsg = document.createElement('div');
    userMsg.className = 'ai-msg ai-msg-user';
    userMsg.innerHTML = `<div class="ai-bubble">${prompt}</div>`;
    copilotChatHistory.appendChild(userMsg);
    copilotChatHistory.scrollTop = copilotChatHistory.scrollHeight;

    try {
      const res = await fetch('/api/dashboard/ai/copilot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prompt })
      });
      const data = await res.json();

      const botMsg = document.createElement('div');
      botMsg.className = 'ai-msg ai-msg-bot';
      botMsg.innerHTML = `
        <div class="ai-avatar">🤖</div>
        <div class="ai-bubble">
          <strong>Action: ${data.action}</strong><br>
          ${data.explanation}
        </div>
      `;
      copilotChatHistory.appendChild(botMsg);
      copilotChatHistory.scrollTop = copilotChatHistory.scrollHeight;
      showToast(`🤖 AI Copilot: ${data.action} executed!`);
    } catch (err) {
      showToast(`❌ Error: ${err.message}`);
    }
  }

  if (sendCopilotBtn && copilotInput) {
    sendCopilotBtn.addEventListener('click', () => {
      const val = copilotInput.value.trim();
      copilotInput.value = '';
      sendHubCopilot(val);
    });
    copilotInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') {
        const val = copilotInput.value.trim();
        copilotInput.value = '';
        sendHubCopilot(val);
      }
    });
  }

  if (quickDiagBtn) {
    quickDiagBtn.addEventListener('click', async () => {
      try {
        const res = await fetch('/api/dashboard/ai/diagnose');
        const diag = await res.json();
        if (copilotChatHistory) {
          const botMsg = document.createElement('div');
          botMsg.className = 'ai-msg ai-msg-bot';
          botMsg.innerHTML = `
            <div class="ai-avatar">🤖</div>
            <div class="ai-bubble">
              <strong>Incident Diagnostic Report:</strong><br>
              • <strong>Status:</strong> <span class="mono ${diag.health_status === 'OPTIMAL' ? 'text-green' : 'text-yellow'}">${diag.health_status} (${diag.health_score}/100)</span><br>
              • <strong>Active RPS:</strong> <span class="mono">${diag.active_rps.toFixed(1)} req/s</span><br>
              • <strong>Findings:</strong><br>${diag.findings.map(f => `  - ${f}`).join('<br>')}<br>
              • <strong>Recommended Action:</strong><br>${diag.remediations.map(r => `  - ${r}`).join('<br>')}
            </div>
          `;
          copilotChatHistory.appendChild(botMsg);
          copilotChatHistory.scrollTop = copilotChatHistory.scrollHeight;
        }
        showToast('🩺 Incident Diagnosis complete!');
      } catch (err) {
        showToast(`❌ Diagnosis failed: ${err.message}`);
      }
    });
  }
}

