// ============================================================
// Routa Dashboard â€” Client Application
// ============================================================

(function () {
    'use strict';

    // --- State ---
    let selectedEntryId = null;
    let ws = null;
    let reconnectTimer = null;

    // --- DOM References ---
    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => document.querySelectorAll(sel);

    // --- Init ---
    document.addEventListener('DOMContentLoaded', () => {
        initTabs();
        initFilters();
        initDetailTabs();
        initButtons();
        initModal();
        connectWebSocket();
        fetchTunnelStatus();
        fetchRequests();
        setInterval(fetchTunnelStatus, 5000);
    });

    // ============================================================
    // Navigation Tabs
    // ============================================================
    function initTabs() {
        $$('.nav-tab').forEach(tab => {
            tab.addEventListener('click', () => {
                $$('.nav-tab').forEach(t => t.classList.remove('active'));
                $$('.tab-content').forEach(t => t.classList.remove('active'));
                tab.classList.add('active');
                const target = $(`#tab-${tab.dataset.tab}`);
                if (target) target.classList.add('active');

                // Load data for the tab
                if (tab.dataset.tab === 'webhooks') fetchWebhooks();
                if (tab.dataset.tab === 'sessions') fetchSessions();
                if (tab.dataset.tab === 'mocklab') fetchMocks();
                if (tab.dataset.tab === 'discovery') fetchDiscovery();
                if (tab.dataset.tab === 'apimap') fetchAPIMap();
            });
        });
    }

    // ============================================================
    // Filter Bar
    // ============================================================
    function initFilters() {
        let debounceTimer;
        const searchInput = $('#filter-search');
        if (searchInput) {
            searchInput.addEventListener('input', () => {
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(fetchRequests, 300);
            });
        }
        const methodSelect = $('#filter-method');
        if (methodSelect) methodSelect.addEventListener('change', fetchRequests);
        const statusSelect = $('#filter-status');
        if (statusSelect) statusSelect.addEventListener('change', fetchRequests);
    }

    function getFilterParams() {
        const params = new URLSearchParams();
        const searchInput = $('#filter-search');
        const methodSelect = $('#filter-method');
        const statusSelect = $('#filter-status');

        const search = searchInput ? searchInput.value : '';
        const method = methodSelect ? methodSelect.value : '';
        const status = statusSelect ? statusSelect.value : '';

        if (search) params.set('search', search);
        if (method) params.set('method', method);
        if (status === '2xx') { params.set('status_min', 200); params.set('status_max', 299); }
        else if (status === '3xx') { params.set('status_min', 300); params.set('status_max', 399); }
        else if (status === '4xx') { params.set('status_min', 400); params.set('status_max', 499); }
        else if (status === '5xx') { params.set('status_min', 500); params.set('status_max', 599); }

        params.set('limit', '100');
        return params.toString();
    }

    // ============================================================
    // Detail Tabs
    // ============================================================
    function initDetailTabs() {
        $$('.detail-tab').forEach(tab => {
            tab.addEventListener('click', () => {
                $$('.detail-tab').forEach(t => t.classList.remove('active'));
                $$('.detail-section').forEach(s => s.classList.remove('active'));
                tab.classList.add('active');
                const target = $(`#detail-${tab.dataset.detail}`);
                if (target) target.classList.add('active');
            });
        });
    }

    // ============================================================
    // Buttons
    // ============================================================
    function initButtons() {
        // Clear requests
        const btnClear = $('#btn-clear');
        if (btnClear) {
            btnClear.addEventListener('click', async () => {
                await fetch('/api/requests', { method: 'DELETE' });
                fetchRequests();
                const detailPanel = $('#detail-panel');
                if (detailPanel) detailPanel.classList.add('hidden');
                selectedEntryId = null;
                showToast('Requests cleared', 'info');
            });
        }

        // Replay
        const btnReplay = $('#btn-replay');
        if (btnReplay) {
            btnReplay.addEventListener('click', async () => {
                if (!selectedEntryId) return;
                try {
                    const resp = await fetch(`/api/requests/${selectedEntryId}/replay`, { method: 'POST' });
                    if (resp.ok) {
                        showToast('Request replayed', 'success');
                    } else {
                        showToast('Replay failed', 'error');
                    }
                } catch (e) {
                    showToast('Replay error: ' + e.message, 'error');
                }
            });
        }

        // Edit & Replay
        const btnEditReplay = $('#btn-edit-replay');
        if (btnEditReplay) {
            btnEditReplay.addEventListener('click', () => {
                if (!selectedEntryId) return;
                openEditModal(selectedEntryId);
            });
        }

        // Copy public URL
        const publicUrl = $('#public-url');
        if (publicUrl) {
            publicUrl.addEventListener('click', () => {
                const urlEl = $('#public-url-text');
                const url = urlEl ? urlEl.textContent : '';
                if (url && url !== '—' && url !== '-') {
                    navigator.clipboard.writeText(url).then(() => {
                        showToast('URL copied to clipboard', 'success');
                    });
                }
            });
        }

        // Universal modal open button delegation
        document.addEventListener('click', (e) => {
            if (!e.target) return;
            if (e.target.closest('#btn-create-webhook')) {
                const o = document.getElementById('webhook-modal-overlay');
                if (o) o.classList.remove('hidden');
            }
            if (e.target.closest('#btn-add-mock')) {
                const o = document.getElementById('mock-modal-overlay');
                if (o) o.classList.remove('hidden');
            }
            if (e.target.closest('#btn-add-route')) {
                const o = document.getElementById('route-modal-overlay');
                if (o) o.classList.remove('hidden');
            }
            if (e.target.closest('#btn-add-mutation')) {
                const o = document.getElementById('mutation-modal-overlay');
                if (o) o.classList.remove('hidden');
            }
            if (e.target.closest('#btn-add-sim')) {
                const o = document.getElementById('sim-modal-overlay');
                if (o) o.classList.remove('hidden');
            }
        });

        // Save session
        const btnSaveSession = $('#btn-save-session');
        if (btnSaveSession) {
            btnSaveSession.addEventListener('click', async () => {
                const name = prompt('Session name:', `session-${Date.now()}`);
                if (!name) return;
                const resp = await fetch('/api/sessions', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name })
                });
                if (resp.ok) {
                    fetchSessions();
                    showToast('Session saved', 'success');
                }
            });
        }
    }

    // ============================================================
    // Modal
    // ============================================================
    function initModal() {
        const btnClose = $('#modal-close');
        if (btnClose) btnClose.addEventListener('click', closeModal);

        const btnCancel = $('#btn-modal-cancel');
        if (btnCancel) btnCancel.addEventListener('click', closeModal);

        const overlay = $('#modal-overlay');
        if (overlay) {
            overlay.addEventListener('click', (e) => {
                if (e.target === overlay) closeModal();
            });
        }

        const btnSend = $('#btn-modal-send');
        if (btnSend) {
            btnSend.addEventListener('click', async () => {
                const modal = $('#edit-replay-modal');
                const req = {
                    original_id: modal ? (modal.dataset.originalId || '') : '',
                    method: $('#edit-method') ? $('#edit-method').value : 'GET',
                    path: $('#edit-path') ? $('#edit-path').value : '/',
                    query: $('#edit-query') ? $('#edit-query').value : '',
                    headers: {},
                    body: ''
                };

                try {
                    const headersStr = $('#edit-headers') ? $('#edit-headers').value : '{}';
                    req.headers = JSON.parse(headersStr || '{}');
                } catch (e) {
                    showToast('Invalid headers JSON', 'error');
                    return;
                }

                const bodyStr = $('#edit-body') ? $('#edit-body').value || '' : '';
                req.body = Array.from(new TextEncoder().encode(bodyStr));

                try {
                    const resp = await fetch('/api/replay', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(req)
                    });
                    if (resp.ok) {
                        closeModal();
                        showToast('Request sent', 'success');
                    }
                } catch (e) {
                    showToast('Send failed: ' + e.message, 'error');
                }
            });
        }
    }

    async function openEditModal(entryId) {
        try {
            const resp = await fetch(`/api/requests/${entryId}`);
            const entry = await resp.json();

            $('#edit-method').value = entry.method || 'GET';
            $('#edit-path').value = entry.path || '/';
            $('#edit-query').value = entry.query || '';
            $('#edit-headers').value = JSON.stringify(entry.request_headers || {}, null, 2);
            $('#edit-body').value = decodeBody(entry.request_body);
            $('#edit-replay-modal').dataset.originalId = entry.id;

            $('#modal-overlay').classList.remove('hidden');
        } catch (e) {
            showToast('Failed to load request details', 'error');
        }
    }

    function closeModal() {
        $('#modal-overlay').classList.add('hidden');
    }

    // ============================================================
    // WebSocket â€” Live Updates
    // ============================================================
    function connectWebSocket() {
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        ws = new WebSocket(`${proto}//${location.host}/api/ws`);

        ws.onopen = () => {
            console.log('[routa] ws connected');
            if (reconnectTimer) {
                clearTimeout(reconnectTimer);
                reconnectTimer = null;
            }
        };

        ws.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                if (msg.type === 'new_request') {
                    prependRequestItem(msg.entry);
                    updateRequestCount();
                }
            } catch (e) {
                console.error('[routa] ws parse error:', e);
            }
        };

        ws.onclose = () => {
            console.log('[routa] ws disconnected, reconnecting...');
            reconnectTimer = setTimeout(connectWebSocket, 2000);
        };

        ws.onerror = () => {
            ws.close();
        };
    }

    // ============================================================
    // API Calls
    // ============================================================
    async function fetchRequests() {
        try {
            const resp = await fetch(`/api/requests?${getFilterParams()}`);
            const data = await resp.json();
            renderRequestList(data.entries || []);
            $('#request-count').textContent = data.total || 0;
        } catch (e) {
            console.error('[routa] fetch requests:', e);
        }
    }

    async function fetchTunnelStatus() {
        try {
            const resp = await fetch('/api/tunnel/status');
            const data = await resp.json();

            const dot = $('.status-dot');
            const label = $('.status-label');

            if (data.state === 'connected') {
                dot.className = 'status-dot connected';
                label.textContent = 'Connected';
            } else if (data.state === 'connecting') {
                dot.className = 'status-dot';
                label.textContent = 'Connecting\u2026';
            } else if (data.state === 'no_relay') {
                dot.className = 'status-dot local';
                label.textContent = 'Local Only';
            } else {
                dot.className = 'status-dot disconnected';
                label.textContent = 'Disconnected';
            }

            if (data.public_url) {
                $('#public-url-text').textContent = data.public_url;
            }

            if (data.local_target) {
                $('#local-target').textContent = data.local_target;
            }

            if (data.request_count !== undefined) {
                $('#request-count').textContent = data.request_count;
            }
        } catch (e) {
            // Dashboard might not be ready yet
        }
    }

    async function fetchRequestDetail(id) {
        try {
            const resp = await fetch(`/api/requests/${id}`);
            const entry = await resp.json();
            renderDetail(entry);
        } catch (e) {
            console.error('[routa] fetch detail:', e);
        }
    }

    async function fetchWebhooks() {
        try {
            const resp = await fetch('/api/webhooks');
            const data = await resp.json();
            renderWebhookList(data.endpoints || []);
        } catch (e) {
            console.error('[routa] fetch webhooks:', e);
        }
    }

    async function fetchSessions() {
        try {
            const resp = await fetch('/api/sessions');
            const data = await resp.json();
            renderSessionList(data.sessions || []);
        } catch (e) {
            console.error('[routa] fetch sessions:', e);
        }
    }

    // ============================================================
    // Rendering â€” Request List
    // ============================================================
    function renderRequestList(entries) {
        const list = $('#request-list');
        if (!entries || entries.length === 0) {
            list.innerHTML = `
                <div class="empty-state" id="empty-state">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.3">
                        <circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>
                    </svg>
                    <p>Waiting for requestsâ€¦</p>
                    <p class="empty-sub">Send a request to your public URL to see it here</p>
                </div>`;
            return;
        }

        list.innerHTML = entries.map(e => createRequestItemHTML(e)).join('');
        attachRequestClickHandlers();
    }

    function prependRequestItem(entry) {
        const list = $('#request-list');
        const emptyState = $('#empty-state');
        if (emptyState) emptyState.remove();

        const div = document.createElement('div');
        div.innerHTML = createRequestItemHTML(entry);
        const item = div.firstElementChild;

        // Animate in
        item.style.opacity = '0';
        item.style.transform = 'translateX(-10px)';
        list.prepend(item);

        requestAnimationFrame(() => {
            item.style.transition = 'opacity 0.3s ease, transform 0.3s ease';
            item.style.opacity = '1';
            item.style.transform = 'translateX(0)';
        });

        attachRequestClickHandlers();
    }

    function createRequestItemHTML(entry) {
        const statusClass = getStatusClass(entry.status_code);
        const methodClass = `method-${entry.method}`;
        const duration = entry.duration_ms !== undefined ? `${entry.duration_ms}ms` : 'â€”';
        const time = new Date(entry.timestamp).toLocaleTimeString();
        const replayTag = entry.is_replay ? '<span class="replay-tag">â†» replay</span>' : '';

        return `
            <div class="request-item ${entry.is_replay ? 'replay-item' : ''}"
                 data-id="${entry.id}" role="button" tabindex="0">
                <span class="method-badge ${methodClass}">${escapeHtml(entry.method)}</span>
                <span class="request-path">${escapeHtml(entry.path)}${replayTag}</span>
                <span class="status-badge ${statusClass}">${entry.status_code || 'â€”'}</span>
                <span class="request-duration">${duration}</span>
                <span class="request-time">${time}</span>
            </div>`;
    }

    function attachRequestClickHandlers() {
        $$('.request-item').forEach(item => {
            item.addEventListener('click', () => {
                $$('.request-item').forEach(i => i.classList.remove('active'));
                item.classList.add('active');
                selectedEntryId = item.dataset.id;
                $('#detail-panel').classList.remove('hidden');
                fetchRequestDetail(item.dataset.id);
            });
        });
    }

    function updateRequestCount() {
        const count = $$('.request-item').length;
        $('#request-count').textContent = count;
    }

    // ============================================================
    // Rendering â€” Detail Panel
    // ============================================================
    function renderDetail(entry) {
        // Header
        const methodEl = $('#detail-method');
        methodEl.textContent = entry.method;
        methodEl.className = `method-badge method-${entry.method}`;

        $('#detail-path').textContent = entry.path + (entry.query ? '?' + entry.query : '');
        const statusEl = $('#detail-status');
        statusEl.textContent = entry.status_code || 'â€”';
        statusEl.className = `status-badge ${getStatusClass(entry.status_code)}`;

        const durationMs = entry.duration_ms || (entry.duration_ms === 0 ? 0 :
            (typeof entry.duration_ms === 'number' ? entry.duration_ms : 'â€”'));
        $('#detail-duration').textContent = typeof durationMs === 'number' ? `${durationMs}ms` : 'â€”';

        // Request headers
        renderKVTable('#detail-req-headers', entry.request_headers);

        // Request body
        $('#detail-req-body').textContent = decodeBody(entry.request_body) || '(empty)';

        // Response headers
        renderKVTable('#detail-resp-headers', entry.response_headers);

        // Response body
        const respBody = decodeBody(entry.response_body);
        const formatted = tryFormatJSON(respBody);
        $('#detail-resp-body').textContent = formatted || '(empty)';

        // Timing
        renderTiming(entry.timing_breakdown);

        // Show request tab by default
        $$('.detail-tab').forEach(t => t.classList.remove('active'));
        $$('.detail-section').forEach(s => s.classList.remove('active'));
        $$('.detail-tab')[0].classList.add('active');
        $$('.detail-section')[0].classList.add('active');
    }

    function renderKVTable(selector, headers) {
        const container = $(selector);
        if (!headers || Object.keys(headers).length === 0) {
            container.innerHTML = '<div class="kv-row"><div class="kv-value" style="grid-column:1/-1;color:var(--text-tertiary)">(none)</div></div>';
            return;
        }

        container.innerHTML = Object.entries(headers).map(([key, vals]) => {
            const value = Array.isArray(vals) ? vals.join(', ') : vals;
            return `<div class="kv-row">
                <div class="kv-key">${escapeHtml(key)}</div>
                <div class="kv-value">${escapeHtml(String(value))}</div>
            </div>`;
        }).join('');
    }

    function renderTiming(timing) {
        const bars = $('#timing-bars');
        if (!timing) {
            bars.innerHTML = '<p style="color:var(--text-tertiary)">No timing data available</p>';
            return;
        }

        const total = timing.total_ms || 1;
        const items = [
            { label: 'DNS Lookup', value: timing.dns_lookup_ms || 0, cls: 'dns' },
            { label: 'TCP Connect', value: timing.tcp_connect_ms || 0, cls: 'connect' },
            { label: 'TLS Handshake', value: timing.tls_handshake_ms || 0, cls: 'tls' },
            { label: 'First Byte', value: timing.first_byte_ms || 0, cls: 'firstbyte' },
            { label: 'Content Transfer', value: timing.content_transfer_ms || 0, cls: 'transfer' },
            { label: 'Total', value: total, cls: 'total' },
        ];

        bars.innerHTML = items.map(item => {
            const pct = Math.max(2, (item.value / total) * 100);
            return `<div class="timing-row">
                <div class="timing-label">${item.label}</div>
                <div class="timing-bar-track">
                    <div class="timing-bar-fill ${item.cls}" style="width:${pct}%"></div>
                </div>
                <div class="timing-value">${item.value}ms</div>
            </div>`;
        }).join('');
    }

    // ============================================================
    // Rendering â€” Webhooks
    // ============================================================
    function renderWebhookList(endpoints) {
        const container = $('#webhook-list');
        if (!container) return;
        if (!endpoints || endpoints.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.3">
                        <path d="M22 12h-4l-3 9L9 3l-3 9H2"/>
                    </svg>
                    <p>No webhook endpoints yet</p>
                    <p class="empty-sub">Create an endpoint to start receiving webhooks</p>
                </div>`;
            container.className = 'empty-state';
            return;
        }

        container.className = '';
        container.innerHTML = endpoints.map(ep => `
            <div class="webhook-card" data-id="${ep.id}">
                <div class="webhook-info">
                    <div style="display:flex;align-items:center;gap:8px">
                        <div class="webhook-name">${escapeHtml(ep.name)}</div>
                        <span class="rule-tag" style="background:rgba(79,70,229,0.1);color:#4f46e5;font-weight:600">${escapeHtml(ep.provider || 'Custom')}</span>
                        <span class="status-dot ${ep.active !== false ? 'connected' : 'disconnected'}" title="${ep.active !== false ? 'Active' : 'Disabled'}"></span>
                    </div>
                    <div class="webhook-path"><code>${escapeHtml(ep.path)}</code></div>
                    <div class="webhook-meta">Created ${new Date(ep.created_at).toLocaleString()}</div>
                </div>
                <div style="display:flex;gap:6px;align-items:center">
                    <button class="btn btn-secondary btn-sm btn-test-webhook" data-id="${ep.id}" title="Send simulated test delivery payload">Test Delivery</button>
                    <button class="btn btn-outline btn-sm btn-toggle-webhook" data-id="${ep.id}">${ep.active !== false ? 'Disable' : 'Enable'}</button>
                    <button class="btn btn-outline btn-sm btn-copy-webhook" data-path="${escapeHtml(ep.path)}">Copy URL</button>
                    <button class="btn btn-ghost btn-sm btn-delete-webhook" data-id="${ep.id}">Delete</button>
                </div>
            </div>
        `).join('');

        // Copy URL buttons
        container.querySelectorAll('.btn-copy-webhook').forEach(btn => {
            btn.addEventListener('click', () => {
                const url = $('#public-url-text').textContent + btn.dataset.path;
                navigator.clipboard.writeText(url).then(() => {
                    showToast('Webhook URL copied', 'success');
                });
            });
        });

        // Toggle buttons
        container.querySelectorAll('.btn-toggle-webhook').forEach(btn => {
            btn.addEventListener('click', async () => {
                await fetch(`/api/webhooks/${btn.dataset.id}/toggle`, { method: 'POST' });
                fetchWebhooks();
                showToast('Webhook connection toggled', 'info');
            });
        });

        // Test delivery buttons
        container.querySelectorAll('.btn-test-webhook').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    const resp = await fetch(`/api/webhooks/${btn.dataset.id}/test`, { method: 'POST' });
                    if (resp.ok) {
                        showToast('Simulated test webhook delivery sent!', 'success');
                        fetchRequests();
                    }
                } catch(e) {
                    showToast('Test delivery error: ' + e.message, 'error');
                }
            });
        });

        // Delete buttons
        container.querySelectorAll('.btn-delete-webhook').forEach(btn => {
            btn.addEventListener('click', async () => {
                await fetch(`/api/webhooks/${btn.dataset.id}`, { method: 'DELETE' });
                fetchWebhooks();
                showToast('Webhook deleted', 'info');
            });
        });
    }

    // ============================================================
    // Rendering â€” Sessions
    // ============================================================
    function renderSessionList(sessions) {
        const container = $('#session-list');
        if (!sessions || sessions.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.3">
                        <path d="M19 21H5a2 2 0 01-2-2V5a2 2 0 012-2h11l5 5v11a2 2 0 01-2 2z"/>
                    </svg>
                    <p>No saved sessions</p>
                    <p class="empty-sub">Save your current captured requests as a replayable session</p>
                </div>`;
            container.className = 'empty-state';
            return;
        }

        container.className = '';
        container.innerHTML = sessions.map(s => `
            <div class="session-card">
                <div class="session-info">
                    <div class="session-name">${escapeHtml(s.name)}</div>
                    <div class="session-meta">${s.entry_count} requests Â· ${new Date(s.created_at).toLocaleString()}</div>
                </div>
                <div class="session-actions">
                    <button class="btn btn-primary btn-sm btn-load-session" data-name="${escapeHtml(s.name)}">Load</button>
                    <button class="btn btn-ghost btn-sm btn-delete-session" data-name="${escapeHtml(s.name)}">Delete</button>
                </div>
            </div>
        `).join('');

        // Load buttons
        container.querySelectorAll('.btn-load-session').forEach(btn => {
            btn.addEventListener('click', async () => {
                const resp = await fetch(`/api/sessions/${encodeURIComponent(btn.dataset.name)}/load`, { method: 'POST' });
                if (resp.ok) {
                    const data = await resp.json();
                    fetchRequests();
                    showToast(`Loaded ${data.loaded} requests from "${btn.dataset.name}"`, 'success');
                    // Switch to inspector tab
                    $$('.nav-tab')[0].click();
                }
            });
        });

        // Delete buttons
        container.querySelectorAll('.btn-delete-session').forEach(btn => {
            btn.addEventListener('click', async () => {
                await fetch(`/api/sessions/${encodeURIComponent(btn.dataset.name)}`, { method: 'DELETE' });
                fetchSessions();
                showToast('Session deleted', 'info');
            });
        });
    }

    // ============================================================
    // Utilities
    // ============================================================
    function getStatusClass(code) {
        if (!code) return '';
        if (code >= 200 && code < 300) return 'status-2xx';
        if (code >= 300 && code < 400) return 'status-3xx';
        if (code >= 400 && code < 500) return 'status-4xx';
        if (code >= 500) return 'status-5xx';
        return '';
    }

    function decodeBody(body) {
        if (!body) return '';
        // body might be base64 encoded bytes or a string
        if (typeof body === 'string') {
            try {
                return atob(body);
            } catch (e) {
                return body;
            }
        }
        // If it's an array of bytes
        if (Array.isArray(body)) {
            try {
                return new TextDecoder().decode(new Uint8Array(body));
            } catch (e) {
                return String(body);
            }
        }
        return String(body);
    }

    function tryFormatJSON(str) {
        if (!str) return str;
        try {
            const parsed = JSON.parse(str);
            return JSON.stringify(parsed, null, 2);
        } catch (e) {
            return str;
        }
    }

    function escapeHtml(str) {
        if (!str) return '';
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    function showToast(message, type = 'info') {
        const container = $('#toast-container');
        const toast = document.createElement('div');
        toast.className = `toast ${type}`;
        toast.textContent = message;
        container.appendChild(toast);

        setTimeout(() => {
            toast.style.animation = 'slideOutRight 0.3s ease forwards';
            setTimeout(() => toast.remove(), 300);
        }, 3000);
    }

})();

// ============================================================
// Phase 2 — Routes, Mutations, Simulator, Diff
// ============================================================

(function() {

    // ---- Tab init for Phase 2 tabs ----
    document.addEventListener('DOMContentLoaded', function() {
        var routesTab = document.querySelector('[data-tab="routes"]');
        var mutTab    = document.querySelector('[data-tab="mutations"]');
        var simTab    = document.querySelector('[data-tab="simulator"]');
        if (routesTab) routesTab.addEventListener('click', fetchRoutes);
        if (mutTab)    mutTab.addEventListener('click', fetchMutations);
        if (simTab)    simTab.addEventListener('click', fetchSimulations);

        var addRouteBtn = document.getElementById('btn-add-route');
        var addMutBtn   = document.getElementById('btn-add-mutation');
        var addSimBtn   = document.getElementById('btn-add-sim');
        if (addRouteBtn) addRouteBtn.addEventListener('click', function() { toggleModal('route-modal-overlay', true); });
        if (addMutBtn)   addMutBtn.addEventListener('click', function() { toggleModal('mutation-modal-overlay', true); });
        if (addSimBtn)   addSimBtn.addEventListener('click', function() { toggleModal('sim-modal-overlay', true); });

        setupPhase2Modals();
    });

    function toggleModal(overlayId, show) {
        var overlay = document.getElementById(overlayId);
        if (overlay) {
            if (show) overlay.classList.remove('hidden');
            else overlay.classList.add('hidden');
        }
    }

    function setupPhase2Modals() {
        // Route modal
        var rc = document.getElementById('route-modal-close');
        var rcan = document.getElementById('btn-route-cancel');
        var rs = document.getElementById('btn-route-save');
        var rOverlay = document.getElementById('route-modal-overlay');
        if (rc) rc.addEventListener('click', function() { toggleModal('route-modal-overlay', false); });
        if (rcan) rcan.addEventListener('click', function() { toggleModal('route-modal-overlay', false); });
        if (rOverlay) rOverlay.addEventListener('click', function(e) { if (e.target === rOverlay) toggleModal('route-modal-overlay', false); });
        if (rs) {
            rs.addEventListener('click', async function() {
                var name = document.getElementById('route-name').value;
                var pattern = document.getElementById('route-pattern').value || '/api/*';
                var target = document.getElementById('route-target').value || 'http://localhost:3001';
                try {
                    var r = await fetch('/api/routes');
                    var data = await r.json();
                    var routes = data.routes || [];
                    routes.push({ pattern: pattern, target: target, name: name });
                    await fetch('/api/routes', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({routes: routes})});
                    toggleModal('route-modal-overlay', false);
                    fetchRoutes();
                    showToast2('Route added', 'success');
                } catch(e) {
                    showToast2('Error: ' + e.message, 'error');
                }
            });
        }

        // Mutation modal
        var mc = document.getElementById('mutation-modal-close');
        var mcan = document.getElementById('btn-mut-cancel');
        var ms = document.getElementById('btn-mut-save');
        var mOverlay = document.getElementById('mutation-modal-overlay');
        if (mc) mc.addEventListener('click', function() { toggleModal('mutation-modal-overlay', false); });
        if (mcan) mcan.addEventListener('click', function() { toggleModal('mutation-modal-overlay', false); });
        if (mOverlay) mOverlay.addEventListener('click', function(e) { if (e.target === mOverlay) toggleModal('mutation-modal-overlay', false); });
        if (ms) {
            ms.addEventListener('click', async function() {
                var name = document.getElementById('mut-name').value || 'Rule';
                var method = document.getElementById('mut-method').value;
                var path = document.getElementById('mut-path').value;
                var hdrStr = document.getElementById('mut-header-set').value;

                var rule = { name: name, match: { path: path, method: method }, request: {}, response: {} };
                if (hdrStr && hdrStr.includes(':')) {
                    var parts = hdrStr.split(':');
                    rule.request.set_headers = {};
                    rule.request.set_headers[parts[0].trim()] = parts.slice(1).join(':').trim();
                }

                try {
                    var r = await fetch('/api/mutations');
                    var data = await r.json();
                    var muts = data.mutations || [];
                    muts.push(rule);
                    await fetch('/api/mutations', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({mutations: muts})});
                    toggleModal('mutation-modal-overlay', false);
                    fetchMutations();
                    showToast2('Mutation rule added', 'success');
                } catch(e) {
                    showToast2('Error: ' + e.message, 'error');
                }
            });
        }

        // Simulator modal
        var sc = document.getElementById('sim-modal-close');
        var scan = document.getElementById('btn-sim-cancel');
        var ss = document.getElementById('btn-sim-save');
        var sOverlay = document.getElementById('sim-modal-overlay');
        if (sc) sc.addEventListener('click', function() { toggleModal('sim-modal-overlay', false); });
        if (scan) scan.addEventListener('click', function() { toggleModal('sim-modal-overlay', false); });
        if (sOverlay) sOverlay.addEventListener('click', function(e) { if (e.target === sOverlay) toggleModal('sim-modal-overlay', false); });
        if (ss) {
            ss.addEventListener('click', async function() {
                var name = document.getElementById('sim-name').value || 'Simulation Rule';
                var method = document.getElementById('sim-method').value;
                var path = document.getElementById('sim-path').value;
                var delay = parseInt(document.getElementById('sim-delay').value || '0');
                var status = parseInt(document.getElementById('sim-status').value || '0');

                var rule = {
                    name: name,
                    match: { path: path, method: method },
                    delay_ms: delay,
                    injected_status: status
                };

                try {
                    var r = await fetch('/api/simulations');
                    var data = await r.json();
                    var sims = data.simulations || [];
                    sims.push(rule);
                    await fetch('/api/simulations', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({simulations: sims})});
                    toggleModal('sim-modal-overlay', false);
                    fetchSimulations();
                    showToast2('Simulation rule added', 'success');
                } catch(e) {
                    showToast2('Error: ' + e.message, 'error');
                }
            });
        }
    }

    // ============================================================
    // Routes
    // ============================================================
    async function fetchRoutes() {
        try {
            var resp = await fetch('/api/routes');
            var data = await resp.json();
            renderRoutes(data.routes || []);
        } catch(e) {
            console.error('[routa] fetch routes:', e);
        }
    }

    function renderRoutes(routes) {
        var el = document.getElementById('routes-list');
        if (!routes.length) {
            el.innerHTML = '<div class="empty-state"><p>No routes configured</p><p class="empty-sub">Routes direct URL patterns to different local services. Default: all traffic to the CLI port.</p></div>';
            return;
        }
        el.innerHTML = routes.map(function(r, i) {
            return '<div class="rule-card">' +
                '<div class="rule-info">' +
                  '<div class="rule-name">' + esc(r.name || 'Route ' + (i+1)) + '</div>' +
                  '<div class="rule-detail">' + esc(r.pattern) + ' &rarr; ' + esc(r.target) + '</div>' +
                '</div>' +
                '<div style="display:flex;gap:6px">' +
                  '<button class="btn btn-ghost btn-sm btn-del-route" data-idx="' + i + '">Delete</button>' +
                '</div>' +
            '</div>';
        }).join('');

        el.querySelectorAll('.btn-del-route').forEach(function(btn) {
            btn.addEventListener('click', async function() {
                var resp2 = await fetch('/api/routes');
                var data2 = await resp2.json();
                var r2 = (data2.routes || []).filter(function(_, j) { return j !== parseInt(btn.dataset.idx); });
                await fetch('/api/routes', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({routes: r2})});
                fetchRoutes();
                showToast2('Route deleted', 'info');
            });
        });
    }

    // ============================================================
    // Mutations
    // ============================================================
    async function fetchMutations() {
        try {
            var resp = await fetch('/api/mutations');
            var data = await resp.json();
            renderMutations(data.mutations || []);
        } catch(e) {
            console.error('[routa] fetch mutations:', e);
        }
    }

    function renderMutations(mutations) {
        var el = document.getElementById('mutations-list');
        if (!mutations.length) {
            el.innerHTML = '<div class="empty-state"><p>No mutation rules</p><p class="empty-sub">Add rules to modify headers, paths, query params, or JSON bodies in-flight</p></div>';
            return;
        }
        el.innerHTML = mutations.map(function(m, i) {
            var tags = [];
            if (m.request && Object.keys(m.request.set_headers || {}).length) tags.push('<span class="rule-tag">+req headers</span>');
            if (m.request && m.request.strip_path_prefix) tags.push('<span class="rule-tag">strip path</span>');
            if (m.response && m.response.mock_status) tags.push('<span class="rule-tag mock">mock ' + m.response.mock_status + '</span>');
            if (m.response && m.response.force_status) tags.push('<span class="rule-tag warn">force ' + m.response.force_status + '</span>');

            var matchStr = (m.match && m.match.method ? m.match.method + ' ' : 'ANY ') + (m.match && m.match.path ? m.match.path : '*');

            return '<div class="rule-card">' +
                '<div class="rule-info">' +
                  '<div class="rule-name">' + esc(m.name || 'Rule ' + (i+1)) + '</div>' +
                  '<div class="rule-match"><span>Matches:</span><code>' + esc(matchStr) + '</code>' + tags.join('') + '</div>' +
                '</div>' +
                '<button class="btn btn-ghost btn-sm btn-del-mut" data-idx="' + i + '">Delete</button>' +
            '</div>';
        }).join('');

        el.querySelectorAll('.btn-del-mut').forEach(function(btn) {
            btn.addEventListener('click', async function() {
                var resp2 = await fetch('/api/mutations');
                var data2 = await resp2.json();
                var m2 = (data2.mutations || []).filter(function(_, j) { return j !== parseInt(btn.dataset.idx); });
                await fetch('/api/mutations', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({mutations: m2})});
                fetchMutations();
                showToast2('Mutation deleted', 'info');
            });
        });
    }

    // ============================================================
    // Simulator
    // ============================================================
    async function fetchSimulations() {
        try {
            var resp = await fetch('/api/simulations');
            var data = await resp.json();
            renderSimulations(data.simulations || []);
        } catch(e) {
            console.error('[routa] fetch sims:', e);
        }
    }

    function renderSimulations(sims) {
        var el = document.getElementById('sim-list');
        if (!sims.length) {
            el.innerHTML = '<div class="empty-state"><p>No simulation rules</p><p class="empty-sub">Inject latency, errors, drops, or timeouts to test how your app handles failures</p></div>';
            return;
        }
        el.innerHTML = sims.map(function(s, i) {
            var tags = [];
            if (s.delay_ms)    tags.push('<span class="rule-tag">' + s.delay_ms + 'ms delay</span>');
            if (s.jitter_ms)   tags.push('<span class="rule-tag">±' + s.jitter_ms + 'ms jitter</span>');
            if (s.error_rate)  tags.push('<span class="rule-tag warn">' + Math.round(s.error_rate*100) + '% errors</span>');
            if (s.drop)        tags.push('<span class="rule-tag drop">drop</span>');
            if (s.timeout_ms)  tags.push('<span class="rule-tag warn">' + s.timeout_ms + 'ms timeout</span>');

            var matchStr = (s.match && s.match.method ? s.match.method + ' ' : 'ANY ') + (s.match && s.match.path ? s.match.path : '*');

            return '<div class="rule-card">' +
                '<div class="rule-info">' +
                  '<div class="rule-name">' + esc(s.name || 'Sim ' + (i+1)) + '</div>' +
                  '<div class="rule-match"><span>Matches:</span><code>' + esc(matchStr) + '</code>' + tags.join('') + '</div>' +
                '</div>' +
                '<button class="btn btn-ghost btn-sm btn-del-sim" data-idx="' + i + '">Delete</button>' +
            '</div>';
        }).join('');

        el.querySelectorAll('.btn-del-sim').forEach(function(btn) {
            btn.addEventListener('click', async function() {
                var resp2 = await fetch('/api/simulations');
                var data2 = await resp2.json();
                var s2 = (data2.simulations || []).filter(function(_, j) { return j !== parseInt(btn.dataset.idx); });
                await fetch('/api/simulations', {method:'PUT', headers:{'Content-Type':'application/json'}, body: JSON.stringify({simulations: s2})});
                fetchSimulations();
                showToast2('Simulation deleted', 'info');
            });
        });
    }

    // ============================================================
    // Diff View
    // ============================================================
    window.loadDiff = async function(entryId) {
        var el = document.getElementById('diff-content');
        if (!el) return;
        try {
            var resp = await fetch('/api/requests/' + entryId + '/diff');
            if (!resp.ok) {
                el.innerHTML = '<div class="diff-none">No diff available — replay this request first</div>';
                return;
            }
            var diff = await resp.json();
            if (!diff) {
                el.innerHTML = '<div class="diff-none">No differences found</div>';
                return;
            }
            var html = '';

            // Status diff
            html += '<div class="diff-section"><h4>Status</h4>';
            if (diff.status_diff) {
                html += '<div class="diff-status">' + esc(diff.status_diff) + '</div>';
            } else {
                html += '<div class="diff-status unchanged">Unchanged</div>';
            }
            html += '</div>';

            // Headers diff
            html += '<div class="diff-section"><h4>Headers</h4>';
            var hkeys = Object.keys(diff.headers_diff || {});
            if (hkeys.length) {
                html += hkeys.map(function(k) {
                    return '<div class="diff-header-item"><div class="diff-header-key">' + esc(k) + '</div><div class="diff-header-val">' + esc(diff.headers_diff[k]) + '</div></div>';
                }).join('');
            } else {
                html += '<div class="diff-none">No header changes</div>';
            }
            html += '</div>';

            // Body diff
            html += '<div class="diff-section"><h4>Body</h4>';
            if (diff.body_diff) {
                var lines = diff.body_diff.split('\n');
                var formatted = lines.map(function(l) {
                    if (l.startsWith('+')) return '<span class="diff-line-add">' + esc(l) + '</span>';
                    if (l.startsWith('-')) return '<span class="diff-line-remove">' + esc(l) + '</span>';
                    if (l.startsWith('~')) return '<span class="diff-line-change">' + esc(l) + '</span>';
                    return esc(l);
                }).join('\n');
                html += '<pre class="diff-body-pre">' + formatted + '</pre>';
            } else {
                html += '<div class="diff-none">Body unchanged</div>';
            }
            html += '</div>';

            el.innerHTML = html;
        } catch(e) {
            el.innerHTML = '<div class="diff-none">Error loading diff: ' + esc(String(e)) + '</div>';
        }
    };

    // Hook into detail tab switches to auto-load diff when switching to Diff tab
    document.addEventListener('DOMContentLoaded', function() {
        document.addEventListener('click', function(e) {
            var tab = e.target.closest('.detail-tab');
            if (tab && tab.dataset.detail === 'diff') {
                // Get currently selected entry
                var active = document.querySelector('.request-item.active');
                if (active && window.loadDiff) window.loadDiff(active.dataset.id);
            }
        });
    });

    // Helpers
    function esc(str) {
        if (!str && str !== 0) return '';
        var d = document.createElement('div');
        d.textContent = String(str);
        return d.innerHTML;
    }

    function showToast2(msg, type) {
        var c = document.getElementById('toast-container');
        if (!c) return;
        var t = document.createElement('div');
        t.className = 'toast ' + (type || 'info');
        t.textContent = msg;
        c.appendChild(t);
        setTimeout(function() {
            t.style.animation = 'slideOutRight 0.3s ease forwards';
            setTimeout(function() { t.remove(); }, 300);
        }, 3000);
    }



    document.addEventListener('DOMContentLoaded', function() {
        var discoveryTab = document.querySelector('[data-tab="discovery"]');
        var apiMapTab    = document.querySelector('[data-tab="apimap"]');
        var mockLabTab   = document.querySelector('[data-tab="mocklab"]');

        if (discoveryTab) discoveryTab.addEventListener('click', fetchDiscovery);
        if (apiMapTab)    apiMapTab.addEventListener('click', fetchAPIMap);
        if (mockLabTab)   mockLabTab.addEventListener('click', fetchMocks);

        var scanBtn = document.getElementById('btn-scan-discovery');
        if (scanBtn) {
            scanBtn.addEventListener('click', async function() {
                scanBtn.disabled = true;
                scanBtn.innerHTML = 'Scanning ports...';
                try {
                    var resp = await fetch('/api/discovery/services', { method: 'POST' });
                    var data = await resp.json();
                    renderDiscoveryServices(data.services || []);
                    showToast3('Port scan completed', 'success');
                } catch (e) {
                    showToast3('Scan failed: ' + e.message, 'error');
                } finally {
                    scanBtn.disabled = false;
                    scanBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg> Scan Ports Now';
                }
            });
        }

        var refreshMapBtn = document.getElementById('btn-refresh-apimap');
        if (refreshMapBtn) refreshMapBtn.addEventListener('click', fetchAPIMap);

        var addMockBtn = document.getElementById('btn-add-mock');
        if (addMockBtn) addMockBtn.addEventListener('click', openMockModal);
        setupMockModal();

        var createWebhookBtn = document.getElementById('btn-create-webhook');
        if (createWebhookBtn) createWebhookBtn.addEventListener('click', openWebhookModal);
        setupWebhookModal();

        var createMockFromReqBtn = document.getElementById('btn-create-mock-from-req');
        if (createMockFromReqBtn) {
            createMockFromReqBtn.addEventListener('click', async function() {
                var active = document.querySelector('.request-item.active');
                if (!active) {
                    showToast3('Select a request first', 'info');
                    return;
                }
                try {
                    var resp = await fetch('/api/mocks', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ from_request_id: active.dataset.id })
                    });
                    if (resp.ok) {
                        showToast3('Converted request to mock endpoint!', 'success');
                        var mockTab = document.querySelector('[data-tab="mocklab"]');
                        if (mockTab) mockTab.click();
                    } else {
                        showToast3('Failed to create mock', 'error');
                    }
                } catch (e) {
                    showToast3('Error: ' + e.message, 'error');
                }
            });
        }
    });

    // ============================================================
    // Discovery
    // ============================================================
    async function fetchDiscovery() {
        try {
            var resp = await fetch('/api/discovery/services');
            var data = await resp.json();
            renderDiscoveryServices(data.services || []);

            var resp2 = await fetch('/api/discovery/proposals');
            var data2 = await resp2.json();
            renderDiscoveryProposals(data2.proposals || []);
        } catch (e) {
            console.error('[routa] fetch discovery:', e);
        }
    }

    function renderDiscoveryServices(services) {
        var el = document.getElementById('discovery-services-list');
        if (!el) return;
        if (!services || services.length === 0) {
            el.className = 'empty-state';
            el.innerHTML = '<p>No active local services found.</p><p class="empty-sub">Click "Scan Ports Now" to search standard dev ports (3000, 8080, 5000, etc.)</p>';
            return;
        }

        el.className = '';
        el.innerHTML = services.map(function(s) {
            return '<div class="rule-card">' +
                '<div class="rule-info">' +
                  '<div class="rule-name" style="display:flex;align-items:center;gap:8px">' +
                     esc(s.friendly_name) +
                     (s.is_routed ? '<span class="rule-tag">routed</span>' : '<span class="rule-tag warn">available</span>') +
                  '</div>' +
                  '<div class="rule-detail">' + esc(s.url) + '</div>' +
                '</div>' +
                '<div style="display:flex;gap:6px">' +
                  '<button class="btn btn-outline btn-sm btn-propose-route" data-port="' + s.port + '" data-url="' + esc(s.url) + '" data-name="' + esc(s.friendly_name) + '">Propose Route</button>' +
                '</div>' +
            '</div>';
        }).join('');

        el.querySelectorAll('.btn-propose-route').forEach(function(btn) {
            btn.addEventListener('click', async function() {
                var pattern = prompt('Path prefix pattern to route to ' + btn.dataset.url + ':', '/api/*');
                if (!pattern) return;
                try {
                    var resp = await fetch('/api/discovery/proposals', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            action: 'propose',
                            path_pattern: pattern,
                            target_url: btn.dataset.url,
                            service_name: btn.dataset.name
                        })
                    });
                    if (resp.ok) {
                        fetchDiscovery();
                        showToast3('Route proposal created! Review in the proposals section.', 'success');
                    }
                } catch (e) {
                    showToast3('Error: ' + e.message, 'error');
                }
            });
        });
    }

    function renderDiscoveryProposals(proposals) {
        var el = document.getElementById('discovery-proposals-list');
        if (!el) return;
        if (!proposals || proposals.length === 0) {
            el.className = 'empty-state';
            el.innerHTML = '<p>No pending route proposals.</p><p class="empty-sub">Routa proposes routing rules automatically without breaking your existing setups.</p>';
            return;
        }

        el.className = '';
        el.innerHTML = proposals.map(function(p) {
            var isConfirmed = p.status === 'confirmed';
            return '<div class="rule-card">' +
                '<div class="rule-info">' +
                  '<div class="rule-name">' + esc(p.service_name || 'Proposed Route') + '</div>' +
                  '<div class="rule-detail">' + esc(p.path_pattern) + ' &rarr; ' + esc(p.target_url) + '</div>' +
                  '<div class="rule-match">Status: <span class="rule-tag ' + (isConfirmed ? '' : 'warn') + '">' + esc(p.status) + '</span></div>' +
                '</div>' +
                (!isConfirmed ?
                '<div style="display:flex;gap:6px">' +
                  '<button class="btn btn-primary btn-sm btn-confirm-proposal" data-id="' + p.id + '">Apply Route</button>' +
                '</div>' : '') +
            '</div>';
        }).join('');

        el.querySelectorAll('.btn-confirm-proposal').forEach(function(btn) {
            btn.addEventListener('click', async function() {
                try {
                    var resp = await fetch('/api/discovery/proposals', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            action: 'confirm',
                            id: btn.dataset.id
                        })
                    });
                    if (resp.ok) {
                        fetchDiscovery();
                        showToast3('Route applied successfully to traffic gateway!', 'success');
                    }
                } catch (e) {
                    showToast3('Error: ' + e.message, 'error');
                }
            });
        });
    }

    // ============================================================
    // API Map
    // ============================================================
    async function fetchAPIMap() {
        try {
            var resp = await fetch('/api/discovery/map');
            var data = await resp.json();
            renderAPIMap(data.endpoints || []);
        } catch (e) {
            console.error('[routa] fetch apimap:', e);
        }
    }

    function renderAPIMap(endpoints) {
        var el = document.getElementById('apimap-list');
        if (!el) return;
        if (!endpoints || endpoints.length === 0) {
            el.className = 'empty-state';
            el.innerHTML = '<p>No endpoints discovered yet.</p><p class="empty-sub">Send requests through Routa to build a real-time normalized API map.</p>';
            return;
        }

        el.className = '';
        el.innerHTML = endpoints.map(function(ep) {
            var methods = (ep.methods || []).map(function(m) {
                return '<span class="method-badge method-' + m + '">' + esc(m) + '</span>';
            }).join(' ');

            return '<div class="rule-card">' +
                '<div class="rule-info">' +
                  '<div class="rule-name" style="display:flex;align-items:center;gap:8px">' +
                     methods + ' <code>' + esc(ep.normalized_path) + '</code>' +
                  '</div>' +
                  '<div class="rule-match" style="margin-top:6px">' +
                     '<span>Hits: <strong>' + (ep.hit_count || 0) + '</strong></span>' +
                     '<span>Avg Latency: <strong>' + (ep.avg_duration_ms || 0) + 'ms</strong></span>' +
                     '<span>Last Status: <strong class="status-badge status-' + Math.floor((ep.last_status||200)/100) + 'xx">' + (ep.last_status || 200) + '</strong></span>' +
                  '</div>' +
                '</div>' +
            '</div>';
        }).join('');
    }

    // ============================================================
    // Mock Lab
    // ============================================================
    async function fetchMocks() {
        try {
            var resp = await fetch('/api/mocks');
            var data = await resp.json();
            renderMocks(data.mocks || []);
        } catch (e) {
            console.error('[routa] fetch mocks:', e);
        }
    }

    function renderMocks(mocks) {
        var el = document.getElementById('mocklab-list');
        if (!el) return;
        if (!mocks || mocks.length === 0) {
            el.className = 'empty-state';
            el.innerHTML = '<p>No mock endpoints configured</p><p class="empty-sub">Create a custom mock or click "Create Mock" on captured traffic in Inspector</p>';
            return;
        }

        el.className = '';
        el.innerHTML = mocks.map(function(m) {
            return '<div class="rule-card">' +
                '<div class="rule-info">' +
                  '<div class="rule-name">' + esc(m.name || 'Mock Endpoint') + '</div>' +
                  '<div class="rule-detail"><span class="method-badge method-' + m.method + '">' + esc(m.method) + '</span> <code>' + esc(m.path) + '</code> &rarr; <span class="status-badge status-' + Math.floor((m.status||200)/100) + 'xx">' + m.status + '</span></div>' +
                  (m.delay_ms ? '<div class="rule-match"><span class="rule-tag">' + m.delay_ms + 'ms simulated delay</span></div>' : '') +
                '</div>' +
                '<div style="display:flex;gap:6px">' +
                  '<button class="btn btn-ghost btn-sm btn-delete-mock" data-id="' + m.id + '">Delete</button>' +
                '</div>' +
            '</div>';
        }).join('');

        el.querySelectorAll('.btn-delete-mock').forEach(function(btn) {
            btn.addEventListener('click', async function() {
                try {
                    await fetch('/api/mocks/' + btn.dataset.id, { method: 'DELETE' });
                    fetchMocks();
                    showToast3('Mock endpoint deleted', 'info');
                } catch (e) {
                    showToast3('Error: ' + e.message, 'error');
                }
            });
        });
    }

    function openMockModal() {
        var overlay = document.getElementById('mock-modal-overlay');
        if (overlay) overlay.classList.remove('hidden');
    }

    function closeMockModal() {
        var overlay = document.getElementById('mock-modal-overlay');
        if (overlay) overlay.classList.add('hidden');
    }

    function setupMockModal() {
        var closeBtn = document.getElementById('mock-modal-close');
        var cancelBtn = document.getElementById('btn-mock-cancel');
        var overlay = document.getElementById('mock-modal-overlay');
        var saveBtn = document.getElementById('btn-mock-save');

        if (closeBtn) closeBtn.addEventListener('click', closeMockModal);
        if (cancelBtn) cancelBtn.addEventListener('click', closeMockModal);
        if (overlay) {
            overlay.addEventListener('click', function(e) {
                if (e.target === overlay) closeMockModal();
            });
        }

        if (saveBtn) {
            saveBtn.addEventListener('click', async function() {
                var name = (document.getElementById('mock-name') || {}).value || 'Mock Endpoint';
                var method = (document.getElementById('mock-method') || {}).value || 'GET';
                var path = (document.getElementById('mock-path') || {}).value || '/api/v1/mock';
                var status = parseInt((document.getElementById('mock-status') || {}).value || '200');
                var delay = parseInt((document.getElementById('mock-delay') || {}).value || '0');
                var body = (document.getElementById('mock-body') || {}).value || '{"status":"ok","mocked":true}';

                try {
                    var resp = await fetch('/api/mocks', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            name: name,
                            path: path,
                            method: method,
                            status: status,
                            body: body,
                            delay_ms: delay,
                            active: true
                        })
                    });
                    if (resp.ok) {
                        closeMockModal();
                        fetchMocks();
                        showToast3('Mock endpoint created!', 'success');
                    } else {
                        showToast3('Failed to create mock', 'error');
                    }
                } catch (e) {
                    showToast3('Error: ' + e.message, 'error');
                }
            });
        }
    }

    function openWebhookModal() {
        var overlay = document.getElementById('webhook-modal-overlay');
        if (overlay) overlay.classList.remove('hidden');
    }

    function closeWebhookModal() {
        var overlay = document.getElementById('webhook-modal-overlay');
        if (overlay) overlay.classList.add('hidden');
    }

    function setupWebhookModal() {
        var closeBtn = document.getElementById('webhook-modal-close');
        var cancelBtn = document.getElementById('btn-wh-cancel');
        var overlay = document.getElementById('webhook-modal-overlay');
        var saveBtn = document.getElementById('btn-wh-save');

        if (closeBtn) closeBtn.addEventListener('click', closeWebhookModal);
        if (cancelBtn) cancelBtn.addEventListener('click', closeWebhookModal);
        if (overlay) {
            overlay.addEventListener('click', function(e) {
                if (e.target === overlay) closeWebhookModal();
            });
        }

        if (saveBtn) {
            saveBtn.addEventListener('click', async function() {
                var name = (document.getElementById('wh-name') || {}).value || 'Webhook Endpoint';
                var provider = (document.getElementById('wh-provider') || {}).value || 'Custom';
                var secret = (document.getElementById('wh-secret') || {}).value || '';

                try {
                    var resp = await fetch('/api/webhooks', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ name: name, provider: provider, secret: secret })
                    });
                    if (resp.ok) {
                        closeWebhookModal();
                        fetchWebhooks();
                        showToast3('Webhook endpoint created!', 'success');
                    } else {
                        showToast3('Failed to create webhook endpoint', 'error');
                    }
                } catch (e) {
                    showToast3('Error: ' + e.message, 'error');
                }
            });
        }
    }

    // Helpers
    function esc(str) {
        if (!str && str !== 0) return '';
        var d = document.createElement('div');
        d.textContent = String(str);
        return d.innerHTML;
    }

    function showToast3(msg, type) {
        var c = document.getElementById('toast-container');
        if (!c) return;
        var t = document.createElement('div');
        t.className = 'toast ' + (type || 'info');
        t.textContent = msg;
        c.appendChild(t);
        setTimeout(function() {
            t.style.animation = 'slideOutRight 0.3s ease forwards';
            setTimeout(function() { t.remove(); }, 300);
        }, 3000);
    }

})();
