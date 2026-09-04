// ============================================================
// Routa Dashboard — Client Application
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
                $(`#tab-${tab.dataset.tab}`).classList.add('active');

                // Load data for the tab
                if (tab.dataset.tab === 'webhooks') fetchWebhooks();
                if (tab.dataset.tab === 'sessions') fetchSessions();
            });
        });
    }

    // ============================================================
    // Filter Bar
    // ============================================================
    function initFilters() {
        let debounceTimer;
        $('#filter-search').addEventListener('input', () => {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(fetchRequests, 300);
        });
        $('#filter-method').addEventListener('change', fetchRequests);
        $('#filter-status').addEventListener('change', fetchRequests);
    }

    function getFilterParams() {
        const params = new URLSearchParams();
        const search = $('#filter-search').value;
        const method = $('#filter-method').value;
        const status = $('#filter-status').value;

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
                $(`#detail-${tab.dataset.detail}`).classList.add('active');
            });
        });
    }

    // ============================================================
    // Buttons
    // ============================================================
    function initButtons() {
        // Clear requests
        $('#btn-clear').addEventListener('click', async () => {
            await fetch('/api/requests', { method: 'DELETE' });
            fetchRequests();
            $('#detail-panel').classList.add('hidden');
            selectedEntryId = null;
            showToast('Requests cleared', 'info');
        });

        // Replay
        $('#btn-replay').addEventListener('click', async () => {
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

        // Edit & Replay
        $('#btn-edit-replay').addEventListener('click', () => {
            if (!selectedEntryId) return;
            openEditModal(selectedEntryId);
        });

        // Copy public URL
        $('#public-url').addEventListener('click', () => {
            const url = $('#public-url-text').textContent;
            if (url && url !== '—') {
                navigator.clipboard.writeText(url).then(() => {
                    showToast('URL copied to clipboard', 'success');
                });
            }
        });

        // Create webhook
        $('#btn-create-webhook').addEventListener('click', async () => {
            const name = prompt('Webhook endpoint name:', 'my-webhook');
            if (!name) return;
            await fetch('/api/webhooks', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name })
            });
            fetchWebhooks();
            showToast('Webhook endpoint created', 'success');
        });

        // Save session
        $('#btn-save-session').addEventListener('click', async () => {
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

    // ============================================================
    // Modal
    // ============================================================
    function initModal() {
        $('#modal-close').addEventListener('click', closeModal);
        $('#btn-modal-cancel').addEventListener('click', closeModal);
        $('#modal-overlay').addEventListener('click', (e) => {
            if (e.target === $('#modal-overlay')) closeModal();
        });

        $('#btn-modal-send').addEventListener('click', async () => {
            const req = {
                original_id: $('#edit-replay-modal').dataset.originalId || '',
                method: $('#edit-method').value,
                path: $('#edit-path').value,
                query: $('#edit-query').value,
                headers: {},
                body: btoa($('#edit-body').value || '')
            };

            try {
                req.headers = JSON.parse($('#edit-headers').value || '{}');
            } catch (e) {
                showToast('Invalid headers JSON', 'error');
                return;
            }

            // Convert body to base64 bytes
            const bodyStr = $('#edit-body').value || '';
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
    // WebSocket — Live Updates
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
                label.textContent = 'Connecting…';
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
    // Rendering — Request List
    // ============================================================
    function renderRequestList(entries) {
        const list = $('#request-list');
        if (!entries || entries.length === 0) {
            list.innerHTML = `
                <div class="empty-state" id="empty-state">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.3">
                        <circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>
                    </svg>
                    <p>Waiting for requests…</p>
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
        const duration = entry.duration_ms !== undefined ? `${entry.duration_ms}ms` : '—';
        const time = new Date(entry.timestamp).toLocaleTimeString();
        const replayTag = entry.is_replay ? '<span class="replay-tag">↻ replay</span>' : '';

        return `
            <div class="request-item ${entry.is_replay ? 'replay-item' : ''}"
                 data-id="${entry.id}" role="button" tabindex="0">
                <span class="method-badge ${methodClass}">${escapeHtml(entry.method)}</span>
                <span class="request-path">${escapeHtml(entry.path)}${replayTag}</span>
                <span class="status-badge ${statusClass}">${entry.status_code || '—'}</span>
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
    // Rendering — Detail Panel
    // ============================================================
    function renderDetail(entry) {
        // Header
        const methodEl = $('#detail-method');
        methodEl.textContent = entry.method;
        methodEl.className = `method-badge method-${entry.method}`;

        $('#detail-path').textContent = entry.path + (entry.query ? '?' + entry.query : '');
        const statusEl = $('#detail-status');
        statusEl.textContent = entry.status_code || '—';
        statusEl.className = `status-badge ${getStatusClass(entry.status_code)}`;

        const durationMs = entry.duration_ms || (entry.duration_ms === 0 ? 0 :
            (typeof entry.duration_ms === 'number' ? entry.duration_ms : '—'));
        $('#detail-duration').textContent = typeof durationMs === 'number' ? `${durationMs}ms` : '—';

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
    // Rendering — Webhooks
    // ============================================================
    function renderWebhookList(endpoints) {
        const container = $('#webhook-list');
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
                    <div class="webhook-name">${escapeHtml(ep.name)}</div>
                    <div class="webhook-path">${escapeHtml(ep.path)}</div>
                    <div class="webhook-meta">Created ${new Date(ep.created_at).toLocaleString()}</div>
                </div>
                <div style="display:flex;gap:6px">
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
    // Rendering — Sessions
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
                    <div class="session-meta">${s.entry_count} requests · ${new Date(s.created_at).toLocaleString()}</div>
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
