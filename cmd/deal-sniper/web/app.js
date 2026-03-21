// Deal Sniper — Frontend Application
'use strict';

const API = '';
let refreshTimer = null;
let priceChart = null;
let currentSimServerID = null;

// --- Navigation ---

document.querySelectorAll('nav button').forEach(btn => {
    btn.addEventListener('click', () => {
        document.querySelectorAll('nav button').forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
        btn.classList.add('active');
        document.getElementById('panel-' + btn.dataset.panel).classList.add('active');

        if (btn.dataset.panel === 'history') loadCPUSelect();
        if (btn.dataset.panel === 'orders') loadOrders();
        if (btn.dataset.panel === 'config') loadConfig();
    });
});

// --- Auth ---

function getAuthToken() {
    return sessionStorage.getItem('ds_auth_token') || '';
}

function showAuthModal() {
    document.getElementById('auth-modal').classList.add('active');
    document.getElementById('auth-token-input').focus();
}

function closeAuthModal() {
    document.getElementById('auth-modal').classList.remove('active');
}

function saveAuthToken() {
    const token = document.getElementById('auth-token-input').value.trim();
    if (token) {
        sessionStorage.setItem('ds_auth_token', token);
        updateAuthUI();
        showToast('Authenticated', 'success');
    }
    document.getElementById('auth-token-input').value = '';
    closeAuthModal();
}

function updateAuthUI() {
    const btn = document.getElementById('auth-btn');
    if (getAuthToken()) {
        btn.classList.add('authenticated');
        btn.title = 'Authenticated (click to re-authenticate)';
    } else {
        btn.classList.remove('authenticated');
        btn.title = 'Authenticate for orders';
    }
}

// Init auth UI
updateAuthUI();

// --- Data Fetching ---

async function fetchJSON(path) {
    const resp = await fetch(API + path);
    if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
    return resp.json();
}

async function fetchJSONAuth(path, options = {}) {
    const token = getAuthToken();
    if (!token) {
        showAuthModal();
        throw new Error('Not authenticated');
    }
    const resp = await fetch(API + path, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + token,
            ...(options.headers || {}),
        },
    });
    if (resp.status === 401) {
        sessionStorage.removeItem('ds_auth_token');
        updateAuthUI();
        showToast('Auth token invalid — please re-authenticate', 'error');
        throw new Error('Unauthorized');
    }
    if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
    return resp.json();
}

// --- Live Deals ---

async function loadDeals() {
    try {
        const records = await fetchJSON('/api/latest');
        renderDealStats(records);
        renderDealsTable(records || []);
        updateTimestamp();
    } catch (err) {
        console.error('Failed to load deals:', err);
    }
}

function renderDealStats(records) {
    if (!records || records.length === 0) {
        document.getElementById('deal-stats').innerHTML = '';
        return;
    }
    const count = records.length;
    const avgPrice = records.reduce((s, r) => s + r.Price, 0) / count;
    const avgScore = records.reduce((s, r) => s + r.Score, 0) / count;
    const bestScore = Math.max(...records.map(r => r.Score));

    document.getElementById('deal-stats').innerHTML = `
        <div class="stat-card">
            <div class="stat-label">Active Deals</div>
            <div class="stat-value">${count}</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Avg Price</div>
            <div class="stat-value" style="color: var(--green);">&euro;${avgPrice.toFixed(0)}</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Avg Score</div>
            <div class="stat-value">${avgScore.toFixed(1)}</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Best Score</div>
            <div class="stat-value" style="color: var(--accent);">${bestScore.toFixed(1)}</div>
        </div>
    `;
}

function renderDealsTable(records) {
    const tbody = document.getElementById('deals-table');
    if (records.length === 0) {
        tbody.innerHTML = '<tr><td colspan="9" class="empty-state">No deals found. The scanner may not have run yet.</td></tr>';
        return;
    }

    tbody.innerHTML = records.map(r => `
        <tr class="clickable" onclick="openSimModal(${r.ServerID})">
            <td style="font-family: 'JetBrains Mono', monospace; font-size: 0.8rem;">${r.ServerID}</td>
            <td>${escapeHtml(r.CPU)}</td>
            <td>${r.RAMSize} GB</td>
            <td>${r.TotalStorageTB.toFixed(2)} TB</td>
            <td>${r.NVMeCount}/${r.DriveCount}</td>
            <td>${escapeHtml(r.Datacenter)}</td>
            <td style="font-weight: 600; color: var(--green);">&euro;${r.Price.toFixed(2)}</td>
            <td>${renderScoreBar(r.Score)}</td>
            <td style="color: var(--text-muted); font-size: 0.8rem;">${formatTime(r.ScannedAt)}</td>
        </tr>
    `).join('');
}

function renderScoreBar(score) {
    const color = score >= 80 ? 'var(--green)' : score >= 60 ? 'var(--yellow)' : 'var(--red)';
    return `
        <div class="score-bar">
            <span style="font-weight: 600; min-width: 3ch;">${score.toFixed(1)}</span>
            <div class="score-bar-track">
                <div class="score-bar-fill" style="width: ${score}%; background: ${color};"></div>
            </div>
        </div>
    `;
}

// --- Price History ---

async function loadCPUSelect() {
    try {
        const stats = await fetchJSON('/api/stats');
        const select = document.getElementById('cpu-select');
        const current = select.value;
        select.innerHTML = '<option value="">Select CPU model...</option>';
        const cpus = Object.keys(stats).sort();
        cpus.forEach(cpu => {
            const opt = document.createElement('option');
            opt.value = cpu;
            opt.textContent = `${cpu} (${stats[cpu].Count} records, avg €${stats[cpu].AvgPrice.toFixed(0)})`;
            select.appendChild(opt);
        });
        if (current && cpus.includes(current)) {
            select.value = current;
        }
    } catch (err) {
        console.error('Failed to load CPU stats:', err);
    }
}

document.getElementById('cpu-select').addEventListener('change', async function() {
    const cpu = this.value;
    if (!cpu) {
        if (priceChart) { priceChart.destroy(); priceChart = null; }
        document.getElementById('cpu-stats').innerHTML = '';
        document.getElementById('history-info').textContent = '';
        return;
    }
    try {
        const [history, stats] = await Promise.all([
            fetchJSON(`/api/history?cpu=${encodeURIComponent(cpu)}&limit=500`),
            fetchJSON(`/api/stats/${encodeURIComponent(cpu)}`)
        ]);
        renderPriceChart(history, cpu);
        renderCPUStats(stats);
    } catch (err) {
        console.error('Failed to load history:', err);
    }
});

function renderPriceChart(records, cpu) {
    if (priceChart) priceChart.destroy();

    // Sort by time ascending
    const sorted = [...records].reverse();
    const labels = sorted.map(r => formatTime(r.ScannedAt));
    const prices = sorted.map(r => r.Price);
    const scores = sorted.map(r => r.Score);

    document.getElementById('history-info').textContent = `${sorted.length} data points`;

    const ctx = document.getElementById('price-chart').getContext('2d');
    priceChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Price (€)',
                    data: prices,
                    borderColor: '#22c55e',
                    backgroundColor: 'rgba(34, 197, 94, 0.1)',
                    fill: true,
                    tension: 0.3,
                    pointRadius: sorted.length > 50 ? 0 : 3,
                    yAxisID: 'y',
                },
                {
                    label: 'Score',
                    data: scores,
                    borderColor: '#f97316',
                    backgroundColor: 'rgba(249, 115, 22, 0.1)',
                    fill: false,
                    tension: 0.3,
                    pointRadius: sorted.length > 50 ? 0 : 3,
                    borderDash: [5, 5],
                    yAxisID: 'y1',
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: { intersect: false, mode: 'index' },
            plugins: {
                legend: { labels: { color: '#9499b3', font: { size: 12 } } },
                tooltip: {
                    backgroundColor: '#1a1d28',
                    borderColor: '#2e3347',
                    borderWidth: 1,
                    titleColor: '#e4e6ef',
                    bodyColor: '#9499b3',
                }
            },
            scales: {
                x: {
                    ticks: { color: '#6b7194', maxTicksLimit: 10, font: { size: 11 } },
                    grid: { color: 'rgba(46, 51, 71, 0.5)' },
                },
                y: {
                    position: 'left',
                    ticks: { color: '#22c55e', callback: v => '€' + v, font: { size: 11 } },
                    grid: { color: 'rgba(46, 51, 71, 0.5)' },
                },
                y1: {
                    position: 'right',
                    ticks: { color: '#f97316', font: { size: 11 } },
                    grid: { drawOnChartArea: false },
                }
            }
        }
    });
}

function renderCPUStats(stats) {
    if (!stats) {
        document.getElementById('cpu-stats').innerHTML = '';
        return;
    }
    document.getElementById('cpu-stats').innerHTML = `
        <div class="stat-card">
            <div class="stat-label">Min Price</div>
            <div class="stat-value" style="color: var(--green);">&euro;${stats.MinPrice.toFixed(2)}</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Max Price</div>
            <div class="stat-value" style="color: var(--red);">&euro;${stats.MaxPrice.toFixed(2)}</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Avg Price</div>
            <div class="stat-value">&euro;${stats.AvgPrice.toFixed(2)}</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Records</div>
            <div class="stat-value">${stats.Count}</div>
        </div>
    `;
}

// --- Simulation Modal ---

async function openSimModal(serverID) {
    currentSimServerID = serverID;
    const modal = document.getElementById('sim-modal');
    const info = document.getElementById('sim-server-info');
    const grid = document.getElementById('sim-grid');
    const breakdown = document.getElementById('sim-breakdown');
    const orderSection = document.getElementById('sim-order-section');

    info.innerHTML = '<div class="spinner"></div> Loading simulation...';
    grid.innerHTML = '';
    breakdown.innerHTML = '';
    orderSection.innerHTML = '';
    modal.classList.add('active');

    try {
        const data = await fetchJSON(`/api/simulate/${serverID}`);
        const r = data.result;

        info.innerHTML = `
            <div style="display: flex; gap: 1rem; flex-wrap: wrap; font-size: 0.85rem; color: var(--text-secondary);">
                <span>Server <strong style="color: var(--text-primary);">#${r.Server.id || serverID}</strong></span>
                <span>CPU: <strong style="color: var(--text-primary);">${escapeHtml(r.Server.cpu || 'N/A')}</strong></span>
                <span>Price: <strong style="color: var(--green);">&euro;${(r.Server.price || 0).toFixed(2)}</strong></span>
                <span>Bottleneck relief: <strong style="color: var(--accent);">${r.Bottleneck}</strong></span>
            </div>
        `;

        grid.innerHTML = renderSimMetric('CPU Utilization', r.CPUBefore, r.CPUAfter, data.health_before.cpu, data.health_after.cpu)
            + renderSimMetric('RAM Utilization', r.RAMBefore, r.RAMAfter, data.health_before.ram, data.health_after.ram)
            + renderSimMetric('Disk Utilization', r.DiskBefore, r.DiskAfter, data.health_before.disk, data.health_after.disk)
            + renderSimMetric('Monthly Cost', r.MonthlyCostBefore, r.MonthlyCostAfter, null, null, true);

        // Try to show score breakdown from latest scan data
        try {
            const latest = await fetchJSON('/api/latest?limit=500');
            const serverData = (latest || []).find(s => s.ServerID === serverID);
            if (serverData && serverData.BreakdownJSON && serverData.BreakdownJSON !== '{}') {
                const bd = JSON.parse(serverData.BreakdownJSON);
                breakdown.innerHTML = renderBreakdown(bd);
            }
        } catch (e) {
            // Non-critical, skip breakdown display
        }

        // Show snipe button
        orderSection.innerHTML = `
            <button class="snipe-btn" onclick="startOrderCheck(${serverID})">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18">
                    <circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/>
                </svg>
                Snipe This Server
            </button>
        `;
    } catch (err) {
        info.innerHTML = `<div style="color: var(--red);">Simulation failed: ${escapeHtml(err.message)}</div>`;
    }
}

function renderBreakdown(bd) {
    const metrics = [
        { label: 'CPU/$', value: bd.CPUPerDollar || 0 },
        { label: 'RAM/$', value: bd.RAMPerDollar || 0 },
        { label: 'Stor/$', value: bd.StoragePerDollar || 0 },
        { label: 'NVMe', value: bd.NVMeBonus || 0 },
        { label: 'CPU Gen', value: bd.CPUGenBonus || 0 },
        { label: 'DC', value: bd.LocalityBonus || 0 },
    ];

    const bars = metrics.map(m => {
        const pct = Math.round(m.value * 100);
        const color = pct >= 80 ? 'var(--green)' : pct >= 50 ? 'var(--yellow)' : 'var(--text-muted)';
        return `
            <div class="breakdown-row">
                <span class="breakdown-label">${m.label}</span>
                <div class="breakdown-bar-track">
                    <div class="breakdown-bar-fill" style="width: ${pct}%; background: ${color};"></div>
                </div>
                <span class="breakdown-value">${m.value.toFixed(2)}</span>
            </div>
        `;
    }).join('');

    return `
        <div class="breakdown-card">
            <div class="breakdown-title">Score Breakdown</div>
            ${bars}
        </div>
    `;
}

function renderSimMetric(label, before, after, healthBefore, healthAfter, isCost) {
    const fmt = isCost ? v => `€${v.toFixed(0)}` : v => `${v.toFixed(1)}%`;
    const healthB = healthBefore ? `<span class="health-label health-${healthBefore}">${healthBefore}</span>` : '';
    const healthA = healthAfter ? `<span class="health-label health-${healthAfter}">${healthAfter}</span>` : '';
    const improved = after < before;
    const arrowColor = improved ? 'var(--green)' : 'var(--red)';

    return `
        <div class="sim-metric">
            <div class="sim-metric-label">${label}</div>
            <div class="sim-values">
                <span class="sim-before">${fmt(before)}</span> ${healthB}
                <span class="sim-arrow" style="color: ${arrowColor};">&rarr;</span>
                <span class="sim-after">${fmt(after)}</span> ${healthA}
            </div>
        </div>
    `;
}

function closeSimModal() {
    document.getElementById('sim-modal').classList.remove('active');
    currentSimServerID = null;
}

// Close modal on overlay click
document.getElementById('sim-modal').addEventListener('click', function(e) {
    if (e.target === this) closeSimModal();
});
document.getElementById('auth-modal').addEventListener('click', function(e) {
    if (e.target === this) closeAuthModal();
});

// Close modal on Escape
document.addEventListener('keydown', e => {
    if (e.key === 'Escape') {
        closeSimModal();
        closeAuthModal();
    }
});

// Enter key in auth input
document.getElementById('auth-token-input').addEventListener('keydown', e => {
    if (e.key === 'Enter') saveAuthToken();
});

// --- Order Flow ---

async function startOrderCheck(serverID) {
    if (!getAuthToken()) {
        showAuthModal();
        return;
    }

    const orderSection = document.getElementById('sim-order-section');
    orderSection.innerHTML = '<div class="spinner"></div> Checking eligibility...';

    try {
        const result = await fetchJSONAuth('/api/order/check', {
            method: 'POST',
            body: JSON.stringify({ server_id: serverID }),
        });

        if (!result.eligible) {
            orderSection.innerHTML = `
                <div class="order-result order-ineligible">
                    <strong>Ineligible</strong>
                    <ul>${(result.reasons || []).map(r => `<li>${escapeHtml(r)}</li>`).join('')}</ul>
                </div>
                <button class="snipe-btn" disabled style="opacity: 0.4; cursor: not-allowed;">
                    Snipe This Server
                </button>
            `;
        } else {
            orderSection.innerHTML = `
                <div class="order-result order-eligible">
                    <strong>Eligible</strong> — Server #${result.server_id} at &euro;${result.price.toFixed(2)}/mo, score ${result.score.toFixed(1)}
                </div>
                <div class="order-confirm">
                    <p style="font-size: 0.85rem; color: var(--text-secondary); margin-bottom: 0.75rem;">
                        This will place a real order with the Hetzner Robot API. The server will be provisioned and billed to your account.
                    </p>
                    <button class="snipe-btn snipe-confirm" onclick="confirmOrder(${serverID})">
                        Confirm Order — &euro;${result.price.toFixed(2)}/mo
                    </button>
                    <button class="snipe-btn snipe-cancel" onclick="cancelOrder()">Cancel</button>
                </div>
            `;
        }
    } catch (err) {
        if (err.message === 'Not authenticated' || err.message === 'Unauthorized') return;
        orderSection.innerHTML = `
            <div class="order-result order-error">Check failed: ${escapeHtml(err.message)}</div>
            <button class="snipe-btn" onclick="startOrderCheck(${serverID})">Retry</button>
        `;
    }
}

async function confirmOrder(serverID) {
    const orderSection = document.getElementById('sim-order-section');
    orderSection.innerHTML = '<div class="spinner"></div> Placing order...';

    try {
        const result = await fetchJSONAuth('/api/order/confirm', {
            method: 'POST',
            body: JSON.stringify({ server_id: serverID }),
        });

        if (result.success) {
            showToast(`Order placed! ${result.message}`, 'success');
            orderSection.innerHTML = `
                <div class="order-result order-success">
                    <strong>Order Placed</strong><br>${escapeHtml(result.message)}
                    ${result.transaction_id ? `<br>Transaction: <code>${escapeHtml(result.transaction_id)}</code>` : ''}
                </div>
            `;
        } else {
            showToast(`Order failed: ${result.message}`, 'error');
            orderSection.innerHTML = `
                <div class="order-result order-error">
                    <strong>Order Failed</strong><br>${escapeHtml(result.message)}
                </div>
                <button class="snipe-btn" onclick="startOrderCheck(${serverID})">Retry Check</button>
            `;
        }

        // Refresh orders panel
        loadOrders();
    } catch (err) {
        if (err.message === 'Not authenticated' || err.message === 'Unauthorized') return;
        showToast(`Order error: ${err.message}`, 'error');
        orderSection.innerHTML = `
            <div class="order-result order-error">Error: ${escapeHtml(err.message)}</div>
            <button class="snipe-btn" onclick="startOrderCheck(${serverID})">Retry</button>
        `;
    }
}

function cancelOrder() {
    const serverID = currentSimServerID;
    document.getElementById('sim-order-section').innerHTML = `
        <button class="snipe-btn" onclick="startOrderCheck(${serverID})">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18">
                <circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/>
            </svg>
            Snipe This Server
        </button>
    `;
}

// --- Toast Notifications ---

function showToast(message, type) {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => { toast.classList.add('toast-fade'); }, 3000);
    setTimeout(() => { toast.remove(); }, 3500);
}

// --- Orders ---

async function loadOrders() {
    try {
        const orders = await fetchJSON('/api/orders');
        const tbody = document.getElementById('orders-table');

        if (!orders || orders.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="empty-state">No order attempts recorded.</td></tr>';
            return;
        }

        tbody.innerHTML = orders.map(o => `
            <tr>
                <td style="color: var(--text-muted); font-size: 0.8rem;">${formatTime(o.attempted_at)}</td>
                <td style="font-family: 'JetBrains Mono', monospace;">${o.server_id}</td>
                <td>${o.score.toFixed(1)}</td>
                <td style="color: var(--green);">&euro;${o.price.toFixed(2)}</td>
                <td><span class="health-label ${o.success ? 'health-HEALTHY' : 'health-CRITICAL'}">${o.success ? 'SUCCESS' : 'FAILED'}</span></td>
                <td style="font-size: 0.8rem;">${escapeHtml(o.message)}</td>
            </tr>
        `).join('');
    } catch (err) {
        console.error('Failed to load orders:', err);
    }
}

// --- Config ---

async function loadConfig() {
    try {
        const cfg = await fetchJSON('/api/config');
        const grid = document.getElementById('config-grid');

        grid.innerHTML = renderConfigSection('Filters', {
            'Min RAM': cfg.filters.MinRAMGB + ' GB',
            'Min CPU Cores': cfg.filters.MinCPUCores,
            'Min Drives': cfg.filters.MinDrives,
            'Min Drive Size': cfg.filters.MinDriveSizeGB + ' GB',
            'Max Price': '€' + cfg.filters.MaxPriceEUR,
            'DC Prefix': cfg.filters.DatacenterPrefix,
        }) + renderConfigSection('Scoring Weights', {
            'CPU': (cfg.scoring.CPUWeight * 100).toFixed(0) + '%',
            'RAM': (cfg.scoring.RAMWeight * 100).toFixed(0) + '%',
            'Storage': (cfg.scoring.StorageWeight * 100).toFixed(0) + '%',
            'NVMe': (cfg.scoring.NVMeWeight * 100).toFixed(0) + '%',
            'CPU Gen': (cfg.scoring.CPUGenWeight * 100).toFixed(0) + '%',
            'Locality': (cfg.scoring.LocalityWeight * 100).toFixed(0) + '%',
        }) + renderConfigSection('Cluster', {
            'CPU (millicores)': cfg.cluster.CPUMillicores,
            'CPU Requested': cfg.cluster.CPURequested,
            'RAM': cfg.cluster.RAMGB + ' GB',
            'RAM Requested': cfg.cluster.RAMRequestedGB + ' GB',
            'Disk': cfg.cluster.DiskGB + ' GB',
            'Disk Used': cfg.cluster.DiskUsedGB + ' GB',
            'Nodes': cfg.cluster.Nodes,
        }) + renderConfigSection('Auto-Order', {
            'Enabled': cfg.order.enabled ? 'Yes' : 'No',
            'Min Score': cfg.order.min_score,
            'Max Price': '€' + cfg.order.max_price_eur,
            'Require Approval': cfg.order.require_approval ? 'Yes' : 'No',
        }) + renderConfigSection('Watch', {
            'Interval': cfg.watch.Interval || 'N/A',
            'Dedup Window': cfg.watch.DedupWindow || 'N/A',
        });
    } catch (err) {
        console.error('Failed to load config:', err);
    }
}

function renderConfigSection(title, data) {
    const rows = Object.entries(data).map(([k, v]) =>
        `<div class="config-row"><span class="config-key">${k}</span><span class="config-val">${v}</span></div>`
    ).join('');
    return `<div class="config-section"><h3>${title}</h3>${rows}</div>`;
}

// --- Helpers ---

function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function formatTime(ts) {
    if (!ts) return '—';
    const d = new Date(ts);
    if (isNaN(d.getTime())) return '—';
    const now = new Date();
    const diffMs = now - d;
    const diffMin = Math.floor(diffMs / 60000);

    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    if (diffMin < 1440) return `${Math.floor(diffMin / 60)}h ago`;
    return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' });
}

function updateTimestamp() {
    const el = document.getElementById('last-update');
    const now = new Date();
    el.textContent = `Updated ${now.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`;
}

// --- Init & Auto-Refresh ---

loadDeals();
refreshTimer = setInterval(loadDeals, 60000);
