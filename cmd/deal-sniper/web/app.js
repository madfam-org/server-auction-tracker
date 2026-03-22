// Deal Sniper — Frontend Application
'use strict';

const API = '';
const REFRESH_INTERVAL = 60; // seconds
let refreshTimer = null;
let countdownTimer = null;
let countdownRemaining = REFRESH_INTERVAL;
let priceChart = null;
let currentSimServerID = null;

// Module-level data store for sorting/filtering without re-fetch
let rawRecords = [];
let previousServerIDs = new Set();

// --- Sort state ---
const sortState = { key: 'Score', dir: 'desc' };

(function loadSortFromURL() {
    const params = new URLSearchParams(window.location.search);
    const sortParam = params.get('sort');
    if (sortParam) {
        const [key, dir] = sortParam.split(':');
        if (key) sortState.key = key;
        if (dir === 'asc' || dir === 'desc') sortState.dir = dir;
    }
})();

// --- Filter state (persisted to localStorage) ---
const filterState = loadFilterState();

function loadFilterState() {
    try {
        const params = new URLSearchParams(window.location.search);
        if (params.has('search') || params.has('dc') || params.has('priceMin') || params.has('priceMax') || params.has('nvme') || params.has('ecc')) {
            return {
                search: params.get('search') || '',
                dc: params.get('dc') || '',
                priceMin: params.get('priceMin') || '',
                priceMax: params.get('priceMax') || '',
                nvmeOnly: params.get('nvme') === '1',
                eccOnly: params.get('ecc') === '1',
            };
        }
        const saved = localStorage.getItem('ds_filters');
        return saved ? JSON.parse(saved) : { search: '', dc: '', priceMin: '', priceMax: '', nvmeOnly: false, eccOnly: false };
    } catch { return { search: '', dc: '', priceMin: '', priceMax: '', nvmeOnly: false, eccOnly: false }; }
}

function saveFilterState() {
    localStorage.setItem('ds_filters', JSON.stringify(filterState));
}

// --- Theme ---

function initTheme() {
    const saved = localStorage.getItem('ds_theme');
    if (saved) {
        document.documentElement.setAttribute('data-theme', saved);
    }
}

function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme');
    const next = current === 'light' ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('ds_theme', next);
    // Update chart colors if active
    if (priceChart) updateChartColors();
}

initTheme();

// --- Navigation ---

document.querySelectorAll('nav button').forEach(btn => {
    btn.addEventListener('click', () => {
        document.querySelectorAll('nav button').forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
        btn.classList.add('active');
        document.getElementById('panel-' + btn.dataset.panel).classList.add('active');

        if (btn.dataset.panel === 'history') loadCPUSelect();
        if (btn.dataset.panel === 'orders') loadOrders();
        if (btn.dataset.panel === 'analytics') loadAnalytics();
        if (btn.dataset.panel === 'config') loadConfig();
    });
});

// --- Auth (Janua SSO) ---

let currentUser = null;

function login() {
    window.location.href = '/auth/login';
}

async function logout() {
    try {
        await fetch('/auth/logout', { method: 'POST' });
    } catch { /* ignore */ }
    currentUser = null;
    updateAuthUI();
    showToast('Logged out', 'info');
}

function showAuthModal() {
    document.getElementById('auth-modal').classList.add('active');
}

function closeAuthModal() {
    document.getElementById('auth-modal').classList.remove('active');
}

async function checkAuthState() {
    try {
        const resp = await fetch('/auth/me');
        if (resp.ok) {
            currentUser = await resp.json();
        } else {
            currentUser = null;
        }
    } catch {
        currentUser = null;
    }
    updateAuthUI();
}

function updateAuthUI() {
    const loginBtn = document.getElementById('sso-login-btn');
    const badge = document.getElementById('user-badge');
    const emailEl = document.getElementById('user-email');

    if (currentUser && currentUser.email) {
        loginBtn.style.display = 'none';
        badge.style.display = 'flex';
        emailEl.textContent = currentUser.email;
    } else {
        loginBtn.style.display = 'flex';
        badge.style.display = 'none';
        emailEl.textContent = '';
    }
}

checkAuthState();

// --- Data Fetching ---

async function fetchJSON(path) {
    const resp = await fetch(API + path);
    if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
    return resp.json();
}

async function fetchJSONAuth(path, options = {}) {
    if (!currentUser) {
        showAuthModal();
        throw new Error('Not authenticated');
    }
    const resp = await fetch(API + path, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...(options.headers || {}),
        },
    });
    if (resp.status === 401) {
        currentUser = null;
        updateAuthUI();
        showToast('Session expired — please login again', 'error');
        throw new Error('Unauthorized');
    }
    if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
    return resp.json();
}

// --- Live Deals ---

async function loadDeals() {
    try {
        const records = await fetchJSON('/api/latest');
        const newRecords = records || [];

        // Detect new/changed servers
        const newIDs = new Set(newRecords.map(r => r.ServerID));
        const addedIDs = new Set();
        if (previousServerIDs.size > 0) {
            for (const id of newIDs) {
                if (!previousServerIDs.has(id)) addedIDs.add(id);
            }
            if (addedIDs.size > 0) {
                showToast(`${addedIDs.size} new deal${addedIDs.size > 1 ? 's' : ''} found`, 'info');
            }
        }
        previousServerIDs = newIDs;

        rawRecords = newRecords;
        populateDatacenterDropdown(newRecords);
        renderDealStats(newRecords);
        renderSortedFilteredTable(addedIDs);
        updateTimestamp();
        resetCountdown();
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

// --- Sorting ---

function sortRecords(records) {
    const key = sortState.key;
    const dir = sortState.dir === 'asc' ? 1 : -1;
    return [...records].sort((a, b) => {
        let va = a[key], vb = b[key];
        if (typeof va === 'string') {
            return dir * va.localeCompare(vb);
        }
        return dir * (va - vb);
    });
}

document.querySelectorAll('th.sortable').forEach(th => {
    th.addEventListener('click', () => {
        const key = th.dataset.sort;
        if (sortState.key === key) {
            sortState.dir = sortState.dir === 'asc' ? 'desc' : 'asc';
        } else {
            sortState.key = key;
            sortState.dir = key === 'Score' || key === 'Price' ? 'desc' : 'asc';
        }
        // Update header styles
        document.querySelectorAll('th.sortable').forEach(h => h.classList.remove('sort-asc', 'sort-desc'));
        th.classList.add(sortState.dir === 'asc' ? 'sort-asc' : 'sort-desc');
        syncFilterURL();
        renderSortedFilteredTable();
    });
});

// --- Filtering ---

function applyFilters(records) {
    return records.filter(r => {
        if (filterState.search) {
            const q = filterState.search.toLowerCase();
            if (!r.CPU.toLowerCase().includes(q) && !r.Datacenter.toLowerCase().includes(q) && !String(r.ServerID).includes(q)) {
                return false;
            }
        }
        if (filterState.dc && r.Datacenter !== filterState.dc) return false;
        if (filterState.priceMin && r.Price < Number(filterState.priceMin)) return false;
        if (filterState.priceMax && r.Price > Number(filterState.priceMax)) return false;
        if (filterState.nvmeOnly && r.NVMeCount === 0) return false;
        if (filterState.eccOnly && !r.is_ecc) return false;
        return true;
    });
}

function populateDatacenterDropdown(records) {
    const select = document.getElementById('filter-dc');
    const current = select.value;
    const dcs = [...new Set(records.map(r => r.Datacenter))].sort();
    select.innerHTML = '<option value="">All Datacenters</option>';
    dcs.forEach(dc => {
        const opt = document.createElement('option');
        opt.value = dc;
        opt.textContent = dc;
        select.appendChild(opt);
    });
    if (filterState.dc && dcs.includes(filterState.dc)) {
        select.value = filterState.dc;
    }
}

function renderSortedFilteredTable(newIDs) {
    const filtered = applyFilters(rawRecords);
    const sorted = sortRecords(filtered);
    renderDealsTable(sorted, newIDs);
}

// Debounced input handler
let filterDebounce = null;
function onFilterChange() {
    clearTimeout(filterDebounce);
    filterDebounce = setTimeout(() => {
        filterState.search = document.getElementById('filter-search').value;
        filterState.dc = document.getElementById('filter-dc').value;
        filterState.priceMin = document.getElementById('filter-price-min').value;
        filterState.priceMax = document.getElementById('filter-price-max').value;
        filterState.nvmeOnly = document.getElementById('filter-nvme').checked;
        filterState.eccOnly = document.getElementById('filter-ecc').checked;
        saveFilterState();
        syncFilterURL();
        renderSortedFilteredTable();
    }, 200);
}

function syncFilterURL() {
    const params = new URLSearchParams();
    if (filterState.search) params.set('search', filterState.search);
    if (filterState.dc) params.set('dc', filterState.dc);
    if (filterState.priceMin) params.set('priceMin', filterState.priceMin);
    if (filterState.priceMax) params.set('priceMax', filterState.priceMax);
    if (filterState.nvmeOnly) params.set('nvme', '1');
    if (filterState.eccOnly) params.set('ecc', '1');
    if (sortState.key !== 'Score' || sortState.dir !== 'desc') {
        params.set('sort', sortState.key + ':' + sortState.dir);
    }
    const qs = params.toString();
    const url = window.location.pathname + (qs ? '?' + qs : '');
    history.replaceState(null, '', url);
}

function shareFilters() {
    syncFilterURL();
    navigator.clipboard.writeText(window.location.href).then(() => {
        showToast('URL copied to clipboard', 'success');
    }).catch(() => {
        showToast('Failed to copy URL', 'error');
    });
}

// Bind filter inputs
['filter-search', 'filter-dc', 'filter-price-min', 'filter-price-max', 'filter-nvme', 'filter-ecc'].forEach(id => {
    const el = document.getElementById(id);
    if (el) el.addEventListener('input', onFilterChange);
    if (el) el.addEventListener('change', onFilterChange);
});

// Restore filter state on load
function restoreFilterUI() {
    document.getElementById('filter-search').value = filterState.search || '';
    document.getElementById('filter-price-min').value = filterState.priceMin || '';
    document.getElementById('filter-price-max').value = filterState.priceMax || '';
    document.getElementById('filter-nvme').checked = filterState.nvmeOnly || false;
    document.getElementById('filter-ecc').checked = filterState.eccOnly || false;
}
restoreFilterUI();

function renderDealsTable(records, newIDs) {
    const tbody = document.getElementById('deals-table');
    if (records.length === 0) {
        tbody.innerHTML = '<tr><td colspan="10" class="empty-state">No deals found. The scanner may not have run yet.</td></tr>';
        return;
    }

    tbody.innerHTML = records.map(r => {
        const isNew = newIDs && newIDs.has(r.ServerID);
        return `
        <tr class="clickable${isNew ? ' row-new' : ''}" onclick="openSimModal(${r.ServerID})">
            <td style="font-family: 'JetBrains Mono', monospace; font-size: 0.8rem;">${r.ServerID}</td>
            <td>${escapeHtml(r.CPU)}</td>
            <td>${r.RAMSize} GB</td>
            <td>${r.TotalStorageTB.toFixed(2)} TB</td>
            <td>${r.NVMeCount}/${r.DriveCount}</td>
            <td>${escapeHtml(r.Datacenter)}${r.is_ecc ? ' <span class="ecc-badge" title="ECC Memory">ECC</span>' : ''}</td>
            <td style="font-weight: 600; color: var(--green);">
                &euro;${r.Price.toFixed(2)}
                ${r.next_reduce > 0 ? `<div class="next-reduce">-&euro; in ${r.next_reduce}h</div>` : ''}
            </td>
            <td>${renderScoreBar(r.Score)}</td>
            <td>${renderDealBadge(r.deal_quality_pct, r.percentile)}</td>
            <td style="color: var(--text-muted); font-size: 0.8rem;">
                ${formatTime(r.ScannedAt)}
                ${r.first_seen ? `<div class="time-on-market">${formatListedAge(r.first_seen)}</div>` : ''}
            </td>
        </tr>`;
    }).join('');
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

function renderDealBadge(qualityPct, percentile) {
    if (qualityPct === undefined || qualityPct === 0) return '<span style="color: var(--text-muted);">\u2014</span>';
    const pct = qualityPct.toFixed(0);
    if (qualityPct >= 20) {
        return `<span class="deal-badge deal-badge-great">Top deal ${pct}% below</span>`;
    }
    if (qualityPct > 0) {
        return `<span class="deal-badge deal-badge-good">${pct}% below avg</span>`;
    }
    return `<span class="deal-badge deal-badge-poor">${Math.abs(pct)}% above avg</span>`;
}

// --- Refresh countdown ---

function resetCountdown() {
    countdownRemaining = REFRESH_INTERVAL;
    updateCountdownBar();
}

function tickCountdown() {
    countdownRemaining = Math.max(0, countdownRemaining - 1);
    updateCountdownBar();
}

function updateCountdownBar() {
    const bar = document.getElementById('refresh-bar');
    if (bar) bar.style.width = ((countdownRemaining / REFRESH_INTERVAL) * 100) + '%';
}

// --- Data export ---

function exportData(format) {
    window.open(API + '/api/export?format=' + encodeURIComponent(format), '_blank');
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
            opt.textContent = `${cpu} (${stats[cpu].Count} records, avg \u20AC${stats[cpu].AvgPrice.toFixed(0)})`;
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

function getChartColors() {
    const style = getComputedStyle(document.documentElement);
    return {
        legend: style.getPropertyValue('--chart-legend').trim() || '#9499b3',
        tooltipBg: style.getPropertyValue('--chart-tooltip-bg').trim() || '#1a1d28',
        tooltipBorder: style.getPropertyValue('--chart-tooltip-border').trim() || '#2e3347',
        tooltipTitle: style.getPropertyValue('--chart-tooltip-title').trim() || '#e4e6ef',
        tooltipBody: style.getPropertyValue('--chart-tooltip-body').trim() || '#9499b3',
        grid: style.getPropertyValue('--chart-grid').trim() || 'rgba(46, 51, 71, 0.5)',
        tick: style.getPropertyValue('--chart-tick').trim() || '#6b7194',
        green: style.getPropertyValue('--green').trim() || '#22c55e',
        accent: style.getPropertyValue('--accent').trim() || '#f97316',
    };
}

function renderPriceChart(records, cpu) {
    if (priceChart) priceChart.destroy();

    const sorted = [...records].reverse();
    const labels = sorted.map(r => formatTime(r.ScannedAt));
    const prices = sorted.map(r => r.Price);
    const scores = sorted.map(r => r.Score);
    const c = getChartColors();

    document.getElementById('history-info').textContent = `${sorted.length} data points`;

    const ctx = document.getElementById('price-chart').getContext('2d');
    priceChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [
                {
                    label: 'Price (\u20AC)',
                    data: prices,
                    borderColor: c.green,
                    backgroundColor: c.green + '1a',
                    fill: true,
                    tension: 0.3,
                    pointRadius: sorted.length > 50 ? 0 : 3,
                    yAxisID: 'y',
                },
                {
                    label: 'Score',
                    data: scores,
                    borderColor: c.accent,
                    backgroundColor: c.accent + '1a',
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
                legend: { labels: { color: c.legend, font: { size: 12 } } },
                tooltip: {
                    backgroundColor: c.tooltipBg,
                    borderColor: c.tooltipBorder,
                    borderWidth: 1,
                    titleColor: c.tooltipTitle,
                    bodyColor: c.tooltipBody,
                }
            },
            scales: {
                x: {
                    ticks: { color: c.tick, maxTicksLimit: 10, font: { size: 11 } },
                    grid: { color: c.grid },
                },
                y: {
                    position: 'left',
                    ticks: { color: c.green, callback: v => '\u20AC' + v, font: { size: 11 } },
                    grid: { color: c.grid },
                },
                y1: {
                    position: 'right',
                    ticks: { color: c.accent, font: { size: 11 } },
                    grid: { drawOnChartArea: false },
                }
            }
        }
    });
}

function updateChartColors() {
    if (!priceChart) return;
    const c = getChartColors();
    priceChart.options.plugins.legend.labels.color = c.legend;
    priceChart.options.plugins.tooltip.backgroundColor = c.tooltipBg;
    priceChart.options.plugins.tooltip.borderColor = c.tooltipBorder;
    priceChart.options.plugins.tooltip.titleColor = c.tooltipTitle;
    priceChart.options.plugins.tooltip.bodyColor = c.tooltipBody;
    priceChart.options.scales.x.ticks.color = c.tick;
    priceChart.options.scales.x.grid.color = c.grid;
    priceChart.options.scales.y.ticks.color = c.green;
    priceChart.options.scales.y.grid.color = c.grid;
    priceChart.options.scales.y1.ticks.color = c.accent;
    priceChart.data.datasets[0].borderColor = c.green;
    priceChart.data.datasets[0].backgroundColor = c.green + '1a';
    priceChart.data.datasets[1].borderColor = c.accent;
    priceChart.data.datasets[1].backgroundColor = c.accent + '1a';
    priceChart.update('none');
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

        // Show score breakdown from stored data
        const serverData = rawRecords.find(s => s.ServerID === serverID);
        if (serverData && serverData.BreakdownJSON && serverData.BreakdownJSON !== '{}') {
            try {
                const bd = JSON.parse(serverData.BreakdownJSON);
                breakdown.innerHTML = renderBreakdown(bd);
            } catch (e) { /* skip */ }
        }

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
        { label: 'Bench/$', value: bd.BenchmarkPerDollar || 0 },
        { label: 'ECC', value: bd.ECCBonus || 0 },
    ].filter(m => m.value > 0 || (m.label !== 'Bench/$' && m.label !== 'ECC'));

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
    const fmt = isCost ? v => `\u20AC${v.toFixed(0)}` : v => `${v.toFixed(1)}%`;
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

document.getElementById('sim-modal').addEventListener('click', function(e) {
    if (e.target === this) closeSimModal();
});
document.getElementById('auth-modal').addEventListener('click', function(e) {
    if (e.target === this) closeAuthModal();
});

document.addEventListener('keydown', e => {
    if (e.key === 'Escape') {
        closeSimModal();
        closeAuthModal();
    }
});

// --- Order Flow ---

async function startOrderCheck(serverID) {
    if (!currentUser) {
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
            'Max Price': '\u20AC' + cfg.filters.MaxPriceEUR,
            'DC Prefix': cfg.filters.DatacenterPrefix,
        }) + renderConfigSection('Scoring Weights', {
            'CPU': (cfg.scoring.CPUWeight * 100).toFixed(0) + '%',
            'RAM': (cfg.scoring.RAMWeight * 100).toFixed(0) + '%',
            'Storage': (cfg.scoring.StorageWeight * 100).toFixed(0) + '%',
            'NVMe': (cfg.scoring.NVMeWeight * 100).toFixed(0) + '%',
            'CPU Gen': (cfg.scoring.CPUGenWeight * 100).toFixed(0) + '%',
            'Locality': (cfg.scoring.LocalityWeight * 100).toFixed(0) + '%',
            'Benchmark': (cfg.scoring.BenchmarkWeight * 100).toFixed(0) + '%',
            'ECC': (cfg.scoring.ECCWeight * 100).toFixed(0) + '%',
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
            'Max Price': '\u20AC' + cfg.order.max_price_eur,
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
    if (!ts) return '\u2014';
    const d = new Date(ts);
    if (isNaN(d.getTime())) return '\u2014';
    const now = new Date();
    const diffMs = now - d;
    const diffMin = Math.floor(diffMs / 60000);

    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    if (diffMin < 1440) return `${Math.floor(diffMin / 60)}h ago`;
    return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' });
}

function formatListedAge(firstSeen) {
    if (!firstSeen) return '';
    const d = new Date(firstSeen);
    if (isNaN(d.getTime())) return '';
    const hours = Math.floor((Date.now() - d.getTime()) / 3600000);
    if (hours < 1) return 'Listed <1h ago';
    if (hours < 24) return `Listed ${hours}h ago`;
    return `Listed ${Math.floor(hours / 24)}d ago`;
}

function updateTimestamp() {
    const el = document.getElementById('last-update');
    const now = new Date();
    el.textContent = `Updated ${now.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}`;
}

// --- Analytics ---

let brandChart = null, dcChart = null, cpuValueChart = null, priceDistChart = null;

async function loadAnalytics() {
    try {
        const data = await fetchJSON('/api/analytics');
        renderBrandChart(data.brand_trends || []);
        renderDCChart(data.dc_volume || []);
        renderCPUValueChart(data.top_value_cpus || []);
        renderPriceDistChart(data.price_buckets || []);
    } catch (err) {
        console.error('Failed to load analytics:', err);
    }
}

function renderBrandChart(trends) {
    if (brandChart) brandChart.destroy();
    const c = getChartColors();

    const dates = [...new Set(trends.map(t => t.date))];
    const amdData = dates.map(d => {
        const entry = trends.find(t => t.date === d && t.brand === 'AMD');
        return entry ? entry.avg_price : null;
    });
    const intelData = dates.map(d => {
        const entry = trends.find(t => t.date === d && t.brand === 'Intel');
        return entry ? entry.avg_price : null;
    });

    const ctx = document.getElementById('brand-chart').getContext('2d');
    brandChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: dates,
            datasets: [
                { label: 'AMD', data: amdData, borderColor: '#ed1c24', backgroundColor: 'rgba(237,28,36,0.1)', fill: true, tension: 0.3, pointRadius: 2 },
                { label: 'Intel', data: intelData, borderColor: '#0071c5', backgroundColor: 'rgba(0,113,197,0.1)', fill: true, tension: 0.3, pointRadius: 2 },
            ]
        },
        options: analyticsChartOptions(c, v => '\u20AC' + v.toFixed(0)),
    });
}

function renderDCChart(dcVolume) {
    if (dcChart) dcChart.destroy();
    const c = getChartColors();

    const ctx = document.getElementById('dc-chart').getContext('2d');
    dcChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: dcVolume.map(d => d.datacenter),
            datasets: [{
                label: 'Unique Servers',
                data: dcVolume.map(d => d.count),
                backgroundColor: c.accent + '80',
                borderColor: c.accent,
                borderWidth: 1,
            }]
        },
        options: analyticsChartOptions(c),
    });
}

function renderCPUValueChart(cpus) {
    if (cpuValueChart) cpuValueChart.destroy();
    const c = getChartColors();

    const ctx = document.getElementById('cpu-value-chart').getContext('2d');
    cpuValueChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: cpus.map(v => v.cpu.length > 25 ? v.cpu.substring(0, 25) + '...' : v.cpu),
            datasets: [{
                label: 'Avg Score',
                data: cpus.map(v => v.avg_score),
                backgroundColor: c.green + '80',
                borderColor: c.green,
                borderWidth: 1,
            }]
        },
        options: {
            ...analyticsChartOptions(c),
            indexAxis: 'y',
        },
    });
}

function renderPriceDistChart(buckets) {
    if (priceDistChart) priceDistChart.destroy();
    const c = getChartColors();

    const ctx = document.getElementById('price-dist-chart').getContext('2d');
    priceDistChart = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: buckets.map(b => '\u20AC' + b.bucket),
            datasets: [{
                label: 'Servers',
                data: buckets.map(b => b.count),
                backgroundColor: c.blue + '80',
                borderColor: c.blue,
                borderWidth: 1,
            }]
        },
        options: analyticsChartOptions(c),
    });
}

function analyticsChartOptions(c, tickCallback) {
    return {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
            legend: { labels: { color: c.legend, font: { size: 11 } } },
            tooltip: {
                backgroundColor: c.tooltipBg,
                borderColor: c.tooltipBorder,
                borderWidth: 1,
                titleColor: c.tooltipTitle,
                bodyColor: c.tooltipBody,
            },
        },
        scales: {
            x: {
                ticks: { color: c.tick, font: { size: 10 }, maxTicksLimit: 8 },
                grid: { color: c.grid },
            },
            y: {
                ticks: { color: c.tick, font: { size: 10 }, callback: tickCallback || (v => v) },
                grid: { color: c.grid },
            },
        },
    };
}

// --- Init & Auto-Refresh ---

loadDeals();
refreshTimer = setInterval(loadDeals, REFRESH_INTERVAL * 1000);
countdownTimer = setInterval(tickCountdown, 1000);
