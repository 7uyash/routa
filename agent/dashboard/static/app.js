// ============================================================
// Routa Dashboard — Client Application (Unified)
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
        initAllModals();
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
                const t = tab.dataset.tab;
                if (t === 'scenarios') fetchScenarios();
                if (t === 'webhooks')  fetchWebhooks();
                if (t === 'sessions')  fetchSessions();
                if (t === 'mocklab')   fetchMocks();
                if (t === 'discovery') fetchDiscovery();
                if (t === 'apimap')    fetchAPIMap();
                if (t === 'routes')    fetchRoutes();
                if (t === 'mutations') fetchMutations();
                if (t === 'simulator') fetchSimulations();
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
        const search = ($('#filter-search') || {}).value || '';
        const method = ($('#filter-method') || {}).value || '';
        const status = ($('#filter-status') || {}).value || '';

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
        document.addEventListener('click', (e) => {
            const tab = e.target.closest('.detail-tab');
            if (!tab) return;

            $$('.detail-tab').forEach(t => t.classList.remove('active'));
            $$('.detail-section').forEach(s => s.classList.remove('active'));
            tab.classList.add('active');
            const target = $(`#detail-${tab.dataset.detail}`);
            if (target) target.classList.add('active');

            // Auto-load diff when switching to Diff tab
            if (tab.dataset.detail === 'diff') {
                const active = document.querySelector('.request-item.active');
                if (active) loadDiff(active.dataset.id);
            }
        });
    }

    // ============================================================
    // Buttons — Single unified registration
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
                    if (resp.ok) showToast('Request replayed', 'success');
                    else showToast('Replay failed', 'error');
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
                const url = ($('#public-url-text') || {}).textContent || '';
                if (url && url !== '—' && url !== '-') {
                    navigator.clipboard.writeText(url).then(() => {
                        showToast('URL copied to clipboard', 'success');
                    });
                }
            });
        }

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

        // Scan discovery ports
        const scanBtn = $('#btn-scan-discovery');
        if (scanBtn) {
            scanBtn.addEventListener('click', async () => {
                scanBtn.disabled = true;
                scanBtn.innerHTML = 'Scanning ports...';
                try {
                    const resp = await fetch('/api/discovery/services', { method: 'POST' });
                    const data = await resp.json();
                    renderDiscoveryServices(data.services || []);
                    showToast('Port scan completed', 'success');
                } catch (e) {
                    showToast('Scan failed: ' + e.message, 'error');
                } finally {
                    scanBtn.disabled = false;
                    scanBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg> Scan Ports Now';
                }
            });
        }

        // Refresh API Map
        const refreshMapBtn = $('#btn-refresh-apimap');
        if (refreshMapBtn) refreshMapBtn.addEventListener('click', fetchAPIMap);

        // Create mock from captured request
        const createMockFromReqBtn = $('#btn-create-mock-from-req');
        if (createMockFromReqBtn) {
            createMockFromReqBtn.addEventListener('click', async () => {
                const active = document.querySelector('.request-item.active');
                if (!active) {
                    showToast('Select a request first', 'info');
                    return;
                }
                try {
                    const resp = await fetch('/api/mocks', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ from_request_id: active.dataset.id })
                    });
                    if (resp.ok) {
                        showToast('Converted request to mock endpoint!', 'success');
                        const mockTab = document.querySelector('[data-tab="mocklab"]');
                        if (mockTab) mockTab.click();
                    } else {
                        showToast('Failed to create mock', 'error');
                    }
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        }

        // --- Modal open buttons (single delegation, no duplicates) ---
        const btnAddMock = $('#btn-add-mock');
        if (btnAddMock) btnAddMock.addEventListener('click', () => openModal('mock-modal-overlay'));

        const btnCreateWebhook = $('#btn-create-webhook');
        if (btnCreateWebhook) btnCreateWebhook.addEventListener('click', () => openModal('webhook-modal-overlay'));

        const btnAddRoute = $('#btn-add-route');
        if (btnAddRoute) btnAddRoute.addEventListener('click', () => openModal('route-modal-overlay'));

        const btnAddMutation = $('#btn-add-mutation');
        if (btnAddMutation) btnAddMutation.addEventListener('click', () => openModal('mutation-modal-overlay'));

        const btnAddSim = $('#btn-add-sim');
        if (btnAddSim) btnAddSim.addEventListener('click', () => openModal('sim-modal-overlay'));

        // --- Target Connection Modal ---
        const targetPill = $('#target-pill');
        if (targetPill) targetPill.addEventListener('click', openTargetModal);

        setupModalCloseHandlers('target-modal-overlay', 'target-modal-close', 'btn-target-cancel');

        const btnTargetSet = $('#btn-target-set');
        if (btnTargetSet) {
            btnTargetSet.addEventListener('click', async () => {
                const targetUrl = ($('#target-input') || {}).value || '';
                await setTarget(targetUrl);
            });
        }

        const btnTargetClear = $('#btn-target-clear');
        if (btnTargetClear) {
            btnTargetClear.addEventListener('click', async () => {
                await setTarget('');
            });
        }
    }

    async function openTargetModal() {
        openModal('target-modal-overlay');
        
        // Fetch current target
        try {
            const r = await fetch('/api/target');
            const data = await r.json();
            if ($('#target-input')) $('#target-input').value = data.target || '';
        } catch(e) {}

        // Fetch discovered services
        const list = $('#target-discovered-list');
        if (list) list.innerHTML = '<div class="empty-state" style="padding:16px;">Scanning local ports...</div>';
        
        try {
            const resp = await fetch('/api/discovery/services', { method: 'POST' });
            const data = await resp.json();
            const services = data.services || [];
            
            if (services.length === 0) {
                if (list) list.innerHTML = '<div class="empty-state" style="padding:16px;">No local services found.</div>';
                return;
            }

            let html = '';
            for (const s of services) {
                const url = `http://localhost:${s.port}`;
                html += `
                <div class="discovery-card" style="margin-bottom:8px; cursor:pointer;" onclick="document.getElementById('target-input').value='${url}'">
                    <div class="disc-port">${s.port}</div>
                    <div class="disc-details">
                        <div class="disc-url">${url}</div>
                        <div class="disc-tech">${s.tech || 'HTTP Service'}</div>
                    </div>
                </div>`;
            }
            if (list) list.innerHTML = html;
        } catch (e) {
            if (list) list.innerHTML = '<div class="empty-state" style="padding:16px;">Failed to scan ports.</div>';
        }
    }

    async function setTarget(targetUrl) {
        try {
            const resp = await fetch('/api/target', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ target: targetUrl })
            });
            if (resp.ok) {
                closeModal('target-modal-overlay');
                $('#local-target').textContent = targetUrl || '—';
                showToast(targetUrl ? `Connected to ${targetUrl}` : 'Target cleared', 'success');
            } else {
                showToast('Failed to set target', 'error');
            }
        } catch (e) {
            showToast('Error: ' + e.message, 'error');
        }
    }

    // ============================================================
    // Modal Helpers
    // ============================================================
    function openModal(overlayId) {
        const overlay = document.getElementById(overlayId);
        if (overlay) overlay.classList.remove('hidden');
    }

    function closeModal(overlayId) {
        const overlay = document.getElementById(overlayId);
        if (overlay) overlay.classList.add('hidden');
    }

    // ============================================================
    // All Modals — Single unified setup
    // ============================================================
    function initAllModals() {
        // --- Edit & Replay Modal ---
        setupModalCloseHandlers('modal-overlay', 'modal-close', 'btn-modal-cancel');
        const btnSend = $('#btn-modal-send');
        if (btnSend) {
            btnSend.addEventListener('click', async () => {
                const modal = $('#edit-replay-modal');
                const req = {
                    original_id: modal ? (modal.dataset.originalId || '') : '',
                    method: ($('#edit-method') || {}).value || 'GET',
                    path: ($('#edit-path') || {}).value || '/',
                    query: ($('#edit-query') || {}).value || '',
                    headers: {},
                    body: ''
                };
                try {
                    const headersStr = ($('#edit-headers') || {}).value || '{}';
                    req.headers = JSON.parse(headersStr || '{}');
                } catch (e) {
                    showToast('Invalid headers JSON', 'error');
                    return;
                }
                const bodyStr = ($('#edit-body') || {}).value || '';
                req.body = Array.from(new TextEncoder().encode(bodyStr));
                try {
                    const resp = await fetch('/api/replay', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(req)
                    });
                    if (resp.ok) {
                        closeModal('modal-overlay');
                        showToast('Request sent', 'success');
                    }
                } catch (e) {
                    showToast('Send failed: ' + e.message, 'error');
                }
            });
        }

        // --- Mock Modal ---
        setupModalCloseHandlers('mock-modal-overlay', 'mock-modal-close', 'btn-mock-cancel');
        const btnMockSave = $('#btn-mock-save');
        if (btnMockSave) {
            btnMockSave.addEventListener('click', async () => {
                const name = ($('#mock-name') || {}).value || 'Mock Endpoint';
                const method = ($('#mock-method') || {}).value || 'GET';
                const path = ($('#mock-path') || {}).value || '/api/v1/mock';
                const status = parseInt(($('#mock-status') || {}).value || '200');
                const delay = parseInt(($('#mock-delay') || {}).value || '0');
                const body = ($('#mock-body') || {}).value || '{"status":"ok","mocked":true}';

                try {
                    const resp = await fetch('/api/mocks', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            name, path, method, status, body,
                            delay_ms: delay, active: true
                        })
                    });
                    if (resp.ok) {
                        closeModal('mock-modal-overlay');
                        fetchMocks();
                        showToast('Mock endpoint created!', 'success');
                    } else {
                        showToast('Failed to create mock', 'error');
                    }
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        }

        // --- Route Modal ---
        setupModalCloseHandlers('route-modal-overlay', 'route-modal-close', 'btn-route-cancel');
        const btnRouteSave = $('#btn-route-save');
        if (btnRouteSave) {
            btnRouteSave.addEventListener('click', async () => {
                const name = ($('#route-name') || {}).value || '';
                const pattern = ($('#route-pattern') || {}).value || '/api/*';
                const target = ($('#route-target') || {}).value || 'http://localhost:3001';
                try {
                    const r = await fetch('/api/routes');
                    const data = await r.json();
                    const routes = data.routes || [];
                    routes.push({ pattern, target, name });
                    await fetch('/api/routes', {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ routes })
                    });
                    closeModal('route-modal-overlay');
                    fetchRoutes();
                    showToast('Route added', 'success');
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        }

        // --- Mutation Modal ---
        setupModalCloseHandlers('mutation-modal-overlay', 'mutation-modal-close', 'btn-mut-cancel');
        const btnMutSave = $('#btn-mut-save');
        if (btnMutSave) {
            btnMutSave.addEventListener('click', async () => {
                const name = ($('#mut-name') || {}).value || 'Rule';
                const method = ($('#mut-method') || {}).value || '';
                const path = ($('#mut-path') || {}).value || '';
                const hdrStr = ($('#mut-header-set') || {}).value || '';

                const rule = { name, match: { path, method }, request: {}, response: {} };
                if (hdrStr && hdrStr.includes(':')) {
                    const parts = hdrStr.split(':');
                    rule.request.set_headers = {};
                    rule.request.set_headers[parts[0].trim()] = parts.slice(1).join(':').trim();
                }

                try {
                    const r = await fetch('/api/mutations');
                    const data = await r.json();
                    const muts = data.mutations || [];
                    muts.push(rule);
                    await fetch('/api/mutations', {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ mutations: muts })
                    });
                    closeModal('mutation-modal-overlay');
                    fetchMutations();
                    showToast('Mutation rule added', 'success');
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        }

        // --- Simulator Modal ---
        setupModalCloseHandlers('sim-modal-overlay', 'sim-modal-close', 'btn-sim-cancel');
        const btnSimSave = $('#btn-sim-save');
        if (btnSimSave) {
            btnSimSave.addEventListener('click', async () => {
                const name = ($('#sim-name') || {}).value || 'Simulation Rule';
                const method = ($('#sim-method') || {}).value || '';
                const path = ($('#sim-path') || {}).value || '';
                const delay = parseInt(($('#sim-delay') || {}).value || '0');
                const status = parseInt(($('#sim-status') || {}).value || '0');

                const rule = {
                    name,
                    match: { path, method },
                    delay_ms: delay,
                    injected_status: status
                };

                try {
                    const r = await fetch('/api/simulations');
                    const data = await r.json();
                    const sims = data.simulations || [];
                    sims.push(rule);
                    await fetch('/api/simulations', {
                        method: 'PUT',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ simulations: sims })
                    });
                    closeModal('sim-modal-overlay');
                    fetchSimulations();
                    showToast('Simulation rule added', 'success');
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        }

        // --- Webhook Modal ---
        setupModalCloseHandlers('webhook-modal-overlay', 'webhook-modal-close', 'btn-wh-cancel');
        const btnWhSave = $('#btn-wh-save');
        if (btnWhSave) {
            btnWhSave.addEventListener('click', async () => {
                const name = ($('#wh-name') || {}).value || 'Webhook Endpoint';
                const provider = ($('#wh-provider') || {}).value || 'Custom';
                const secret = ($('#wh-secret') || {}).value || '';

                try {
                    const resp = await fetch('/api/webhooks', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ name, provider, secret })
                    });
                    if (resp.ok) {
                        closeModal('webhook-modal-overlay');
                        fetchWebhooks();
                        showToast('Webhook endpoint created!', 'success');
                    } else {
                        showToast('Failed to create webhook endpoint', 'error');
                    }
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        }
    }

    /**
     * Helper: wire up close (X), cancel button, and overlay-click-to-close
     * for a given modal overlay.
     */
    function setupModalCloseHandlers(overlayId, closeBtnId, cancelBtnId) {
        const overlay = document.getElementById(overlayId);
        const closeBtn = document.getElementById(closeBtnId);
        const cancelBtn = document.getElementById(cancelBtnId);

        if (closeBtn) closeBtn.addEventListener('click', () => closeModal(overlayId));
        if (cancelBtn) cancelBtn.addEventListener('click', () => closeModal(overlayId));
        if (overlay) {
            overlay.addEventListener('click', (e) => {
                if (e.target === overlay) closeModal(overlayId);
            });
        }
    }

    // ============================================================
    // Edit & Replay Modal opener
    // ============================================================
    async function openEditModal(entryId) {
        try {
            const resp = await fetch(`/api/requests/${entryId}`);
            const entry = await resp.json();

            if ($('#edit-method')) $('#edit-method').value = entry.method || 'GET';
            if ($('#edit-path')) $('#edit-path').value = entry.path || '/';
            if ($('#edit-query')) $('#edit-query').value = entry.query || '';
            if ($('#edit-headers')) $('#edit-headers').value = JSON.stringify(entry.request_headers || {}, null, 2);
            if ($('#edit-body')) $('#edit-body').value = decodeBody(entry.request_body);
            if ($('#edit-replay-modal')) $('#edit-replay-modal').dataset.originalId = entry.id;

            openModal('modal-overlay');
        } catch (e) {
            showToast('Failed to load request details', 'error');
        }
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
            // Only update local-target if it's not already set to something else manually by the user
            if (data.local_target !== undefined) {
                $('#local-target').textContent = data.local_target || '—';
                
                // Show modal automatically if no target is set and modal isn't already open
                if (!data.local_target && $('#target-modal-overlay') && $('#target-modal-overlay').classList.contains('hidden')) {
                    // Only pop it once per session to avoid annoying the user
                    if (!window.__targetModalShown) {
                        window.__targetModalShown = true;
                        openTargetModal();
                    }
                }
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

    async function fetchRoutes() {
        try {
            const resp = await fetch('/api/routes');
            const data = await resp.json();
            renderRoutes(data.routes || []);
        } catch (e) {
            console.error('[routa] fetch routes:', e);
        }
    }

    async function fetchMutations() {
        try {
            const resp = await fetch('/api/mutations');
            const data = await resp.json();
            renderMutations(data.mutations || []);
        } catch (e) {
            console.error('[routa] fetch mutations:', e);
        }
    }

    async function fetchSimulations() {
        try {
            const resp = await fetch('/api/simulations');
            const data = await resp.json();
            renderSimulations(data.simulations || []);
        } catch (e) {
            console.error('[routa] fetch sims:', e);
        }
    }

    async function fetchDiscovery() {
        try {
            const resp = await fetch('/api/discovery/services');
            const data = await resp.json();
            renderDiscoveryServices(data.services || []);

            const resp2 = await fetch('/api/discovery/proposals');
            const data2 = await resp2.json();
            renderDiscoveryProposals(data2.proposals || []);
        } catch (e) {
            console.error('[routa] fetch discovery:', e);
        }
    }

    async function fetchAPIMap() {
        try {
            const resp = await fetch('/api/discovery/map');
            const data = await resp.json();
            renderAPIMap(data.endpoints || []);
        } catch (e) {
            console.error('[routa] fetch apimap:', e);
        }
    }

    async function fetchMocks() {
        try {
            const resp = await fetch('/api/mocks');
            const data = await resp.json();
            renderMocks(data.mocks || []);
        } catch (e) {
            console.error('[routa] fetch mocks:', e);
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

        renderKVTable('#detail-req-headers', entry.request_headers);
        $('#detail-req-body').textContent = decodeBody(entry.request_body) || '(empty)';
        renderKVTable('#detail-resp-headers', entry.response_headers);

        const respBody = decodeBody(entry.response_body);
        const formatted = tryFormatJSON(respBody);
        $('#detail-resp-body').textContent = formatted || '(empty)';

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

        container.querySelectorAll('.btn-copy-webhook').forEach(btn => {
            btn.addEventListener('click', () => {
                const url = $('#public-url-text').textContent + btn.dataset.path;
                navigator.clipboard.writeText(url).then(() => showToast('Webhook URL copied', 'success'));
            });
        });

        container.querySelectorAll('.btn-toggle-webhook').forEach(btn => {
            btn.addEventListener('click', async () => {
                await fetch(`/api/webhooks/${btn.dataset.id}/toggle`, { method: 'POST' });
                fetchWebhooks();
                showToast('Webhook connection toggled', 'info');
            });
        });

        container.querySelectorAll('.btn-test-webhook').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    const resp = await fetch(`/api/webhooks/${btn.dataset.id}/test`, { method: 'POST' });
                    if (resp.ok) {
                        showToast('Simulated test webhook delivery sent!', 'success');
                        fetchRequests();
                    }
                } catch (e) {
                    showToast('Test delivery error: ' + e.message, 'error');
                }
            });
        });

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

        container.querySelectorAll('.btn-load-session').forEach(btn => {
            btn.addEventListener('click', async () => {
                const resp = await fetch(`/api/sessions/${encodeURIComponent(btn.dataset.name)}/load`, { method: 'POST' });
                if (resp.ok) {
                    const data = await resp.json();
                    fetchRequests();
                    showToast(`Loaded ${data.loaded} requests from "${btn.dataset.name}"`, 'success');
                    $$('.nav-tab')[0].click();
                }
            });
        });

        container.querySelectorAll('.btn-delete-session').forEach(btn => {
            btn.addEventListener('click', async () => {
                await fetch(`/api/sessions/${encodeURIComponent(btn.dataset.name)}`, { method: 'DELETE' });
                fetchSessions();
                showToast('Session deleted', 'info');
            });
        });
    }

    // ============================================================
    // Rendering — Routes
    // ============================================================
    function renderRoutes(routes) {
        const el = $('#routes-list');
        if (!routes.length) {
            el.innerHTML = '<div class="empty-state"><p>No routes configured</p><p class="empty-sub">Routes direct URL patterns to different local services. Default: all traffic to the CLI port.</p></div>';
            return;
        }
        el.innerHTML = routes.map((r, i) => `
            <div class="rule-card">
                <div class="rule-info">
                    <div class="rule-name">${escapeHtml(r.name || 'Route ' + (i + 1))}</div>
                    <div class="rule-detail">${escapeHtml(r.pattern)} &rarr; ${escapeHtml(r.target)}</div>
                </div>
                <div style="display:flex;gap:6px">
                    <button class="btn btn-ghost btn-sm btn-del-route" data-idx="${i}">Delete</button>
                </div>
            </div>`).join('');

        el.querySelectorAll('.btn-del-route').forEach(btn => {
            btn.addEventListener('click', async () => {
                const resp2 = await fetch('/api/routes');
                const data2 = await resp2.json();
                const r2 = (data2.routes || []).filter((_, j) => j !== parseInt(btn.dataset.idx));
                await fetch('/api/routes', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ routes: r2 }) });
                fetchRoutes();
                showToast('Route deleted', 'info');
            });
        });
    }

    // ============================================================
    // Rendering — Mutations
    // ============================================================
    function renderMutations(mutations) {
        const el = $('#mutations-list');
        if (!mutations.length) {
            el.innerHTML = '<div class="empty-state"><p>No mutation rules</p><p class="empty-sub">Add rules to modify headers, paths, query params, or JSON bodies in-flight</p></div>';
            return;
        }
        el.innerHTML = mutations.map((m, i) => {
            const tags = [];
            if (m.request && Object.keys(m.request.set_headers || {}).length) tags.push('<span class="rule-tag">+req headers</span>');
            if (m.request && m.request.strip_path_prefix) tags.push('<span class="rule-tag">strip path</span>');
            if (m.response && m.response.mock_status) tags.push(`<span class="rule-tag mock">mock ${m.response.mock_status}</span>`);
            if (m.response && m.response.force_status) tags.push(`<span class="rule-tag warn">force ${m.response.force_status}</span>`);
            const matchStr = (m.match && m.match.method ? m.match.method + ' ' : 'ANY ') + (m.match && m.match.path ? m.match.path : '*');

            return `<div class="rule-card">
                <div class="rule-info">
                    <div class="rule-name">${escapeHtml(m.name || 'Rule ' + (i + 1))}</div>
                    <div class="rule-match"><span>Matches:</span><code>${escapeHtml(matchStr)}</code>${tags.join('')}</div>
                </div>
                <button class="btn btn-ghost btn-sm btn-del-mut" data-idx="${i}">Delete</button>
            </div>`;
        }).join('');

        el.querySelectorAll('.btn-del-mut').forEach(btn => {
            btn.addEventListener('click', async () => {
                const resp2 = await fetch('/api/mutations');
                const data2 = await resp2.json();
                const m2 = (data2.mutations || []).filter((_, j) => j !== parseInt(btn.dataset.idx));
                await fetch('/api/mutations', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ mutations: m2 }) });
                fetchMutations();
                showToast('Mutation deleted', 'info');
            });
        });
    }

    // ============================================================
    // Rendering — Simulator
    // ============================================================
    function renderSimulations(sims) {
        const el = $('#sim-list');
        if (!sims.length) {
            el.innerHTML = '<div class="empty-state"><p>No simulation rules</p><p class="empty-sub">Inject latency, errors, drops, or timeouts to test how your app handles failures</p></div>';
            return;
        }
        el.innerHTML = sims.map((s, i) => {
            const tags = [];
            if (s.delay_ms) tags.push(`<span class="rule-tag">${s.delay_ms}ms delay</span>`);
            if (s.jitter_ms) tags.push(`<span class="rule-tag">±${s.jitter_ms}ms jitter</span>`);
            if (s.error_rate) tags.push(`<span class="rule-tag warn">${Math.round(s.error_rate * 100)}% errors</span>`);
            if (s.drop) tags.push('<span class="rule-tag drop">drop</span>');
            if (s.timeout_ms) tags.push(`<span class="rule-tag warn">${s.timeout_ms}ms timeout</span>`);
            const matchStr = (s.match && s.match.method ? s.match.method + ' ' : 'ANY ') + (s.match && s.match.path ? s.match.path : '*');

            return `<div class="rule-card">
                <div class="rule-info">
                    <div class="rule-name">${escapeHtml(s.name || 'Sim ' + (i + 1))}</div>
                    <div class="rule-match"><span>Matches:</span><code>${escapeHtml(matchStr)}</code>${tags.join('')}</div>
                </div>
                <button class="btn btn-ghost btn-sm btn-del-sim" data-idx="${i}">Delete</button>
            </div>`;
        }).join('');

        el.querySelectorAll('.btn-del-sim').forEach(btn => {
            btn.addEventListener('click', async () => {
                const resp2 = await fetch('/api/simulations');
                const data2 = await resp2.json();
                const s2 = (data2.simulations || []).filter((_, j) => j !== parseInt(btn.dataset.idx));
                await fetch('/api/simulations', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ simulations: s2 }) });
                fetchSimulations();
                showToast('Simulation deleted', 'info');
            });
        });
    }

    // ============================================================
    // Rendering — Discovery
    // ============================================================
    function renderDiscoveryServices(services) {
        const el = $('#discovery-services-list');
        if (!el) return;
        if (!services || services.length === 0) {
            el.className = 'empty-state';
            el.innerHTML = '<p>No active local services found.</p><p class="empty-sub">Click "Scan Ports Now" to search standard dev ports (3000, 8080, 5000, etc.)</p>';
            return;
        }

        el.className = '';
        el.innerHTML = services.map(s => `
            <div class="rule-card">
                <div class="rule-info">
                    <div class="rule-name" style="display:flex;align-items:center;gap:8px">
                        ${escapeHtml(s.friendly_name)}
                        ${s.is_routed ? '<span class="rule-tag">routed</span>' : '<span class="rule-tag warn">available</span>'}
                    </div>
                    <div class="rule-detail">${escapeHtml(s.url)}</div>
                </div>
                <div style="display:flex;gap:6px">
                    <button class="btn btn-outline btn-sm btn-propose-route" data-port="${s.port}" data-url="${escapeHtml(s.url)}" data-name="${escapeHtml(s.friendly_name)}">Propose Route</button>
                </div>
            </div>`).join('');

        el.querySelectorAll('.btn-propose-route').forEach(btn => {
            btn.addEventListener('click', async () => {
                const pattern = prompt('Path prefix pattern to route to ' + btn.dataset.url + ':', '/api/*');
                if (!pattern) return;
                try {
                    const resp = await fetch('/api/discovery/proposals', {
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
                        showToast('Route proposal created! Review in the proposals section.', 'success');
                    }
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        });
    }

    function renderDiscoveryProposals(proposals) {
        const el = $('#discovery-proposals-list');
        if (!el) return;
        if (!proposals || proposals.length === 0) {
            el.className = 'empty-state';
            el.innerHTML = '<p>No pending route proposals.</p><p class="empty-sub">Routa proposes routing rules automatically without breaking your existing setups.</p>';
            return;
        }

        el.className = '';
        el.innerHTML = proposals.map(p => {
            const isConfirmed = p.status === 'confirmed';
            return `<div class="rule-card">
                <div class="rule-info">
                    <div class="rule-name">${escapeHtml(p.service_name || 'Proposed Route')}</div>
                    <div class="rule-detail">${escapeHtml(p.path_pattern)} &rarr; ${escapeHtml(p.target_url)}</div>
                    <div class="rule-match">Status: <span class="rule-tag ${isConfirmed ? '' : 'warn'}">${escapeHtml(p.status)}</span></div>
                </div>
                ${!isConfirmed ? `<div style="display:flex;gap:6px"><button class="btn btn-primary btn-sm btn-confirm-proposal" data-id="${p.id}">Apply Route</button></div>` : ''}
            </div>`;
        }).join('');

        el.querySelectorAll('.btn-confirm-proposal').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    const resp = await fetch('/api/discovery/proposals', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ action: 'confirm', id: btn.dataset.id })
                    });
                    if (resp.ok) {
                        fetchDiscovery();
                        showToast('Route applied successfully to traffic gateway!', 'success');
                    }
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        });
    }

    // ============================================================
    // Rendering — API Map
    // ============================================================
    function renderAPIMap(endpoints) {
        const el = $('#apimap-list');
        if (!el) return;
        if (!endpoints || endpoints.length === 0) {
            el.className = 'empty-state';
            el.innerHTML = '<p>No endpoints discovered yet.</p><p class="empty-sub">Send requests through Routa to build a real-time normalized API map.</p>';
            return;
        }

        el.className = '';
        el.innerHTML = endpoints.map(ep => {
            const methods = (ep.methods || []).map(m =>
                `<span class="method-badge method-${m}">${escapeHtml(m)}</span>`
            ).join(' ');

            return `<div class="rule-card">
                <div class="rule-info">
                    <div class="rule-name" style="display:flex;align-items:center;gap:8px">
                        ${methods} <code>${escapeHtml(ep.normalized_path)}</code>
                    </div>
                    <div class="rule-match" style="margin-top:6px">
                        <span>Hits: <strong>${ep.hit_count || 0}</strong></span>
                        <span>Avg Latency: <strong>${ep.avg_duration_ms || 0}ms</strong></span>
                        <span>Last Status: <strong class="status-badge status-${Math.floor((ep.last_status || 200) / 100)}xx">${ep.last_status || 200}</strong></span>
                    </div>
                </div>
            </div>`;
        }).join('');
    }

    // ============================================================
    // Rendering — Mock Lab
    // ============================================================
    function renderMocks(mocks) {
        const el = $('#mocklab-list');
        if (!el) return;
        if (!mocks || mocks.length === 0) {
            el.className = 'empty-state';
            el.innerHTML = '<p>No mock endpoints configured</p><p class="empty-sub">Create a custom mock or click "Create Mock" on captured traffic in Inspector</p>';
            return;
        }

        el.className = '';
        el.innerHTML = mocks.map(m => `
            <div class="rule-card">
                <div class="rule-info">
                    <div class="rule-name">${escapeHtml(m.name || 'Mock Endpoint')}</div>
                    <div class="rule-detail"><span class="method-badge method-${m.method}">${escapeHtml(m.method)}</span> <code>${escapeHtml(m.path)}</code> &rarr; <span class="status-badge status-${Math.floor((m.status || 200) / 100)}xx">${m.status}</span></div>
                    ${m.delay_ms ? `<div class="rule-match"><span class="rule-tag">${m.delay_ms}ms simulated delay</span></div>` : ''}
                </div>
                <div style="display:flex;gap:6px">
                    <button class="btn btn-ghost btn-sm btn-delete-mock" data-id="${m.id}">Delete</button>
                </div>
            </div>`).join('');

        el.querySelectorAll('.btn-delete-mock').forEach(btn => {
            btn.addEventListener('click', async () => {
                try {
                    await fetch('/api/mocks/' + btn.dataset.id, { method: 'DELETE' });
                    fetchMocks();
                    showToast('Mock endpoint deleted', 'info');
                } catch (e) {
                    showToast('Error: ' + e.message, 'error');
                }
            });
        });
    }

    // ============================================================
    // Diff View
    // ============================================================
    async function loadDiff(entryId) {
        const el = $('#diff-content');
        if (!el) return;
        try {
            const resp = await fetch('/api/requests/' + entryId + '/diff');
            if (!resp.ok) {
                el.innerHTML = '<div class="diff-none">No diff available — replay this request first</div>';
                return;
            }
            const diff = await resp.json();
            if (!diff) {
                el.innerHTML = '<div class="diff-none">No differences found</div>';
                return;
            }
            let html = '';

            // Status diff
            html += '<div class="diff-section"><h4>Status</h4>';
            if (diff.status_diff) {
                html += '<div class="diff-status">' + escapeHtml(diff.status_diff) + '</div>';
            } else {
                html += '<div class="diff-status unchanged">Unchanged</div>';
            }
            html += '</div>';

            // Headers diff
            html += '<div class="diff-section"><h4>Headers</h4>';
            const hkeys = Object.keys(diff.headers_diff || {});
            if (hkeys.length) {
                html += hkeys.map(k =>
                    '<div class="diff-header-item"><div class="diff-header-key">' + escapeHtml(k) + '</div><div class="diff-header-val">' + escapeHtml(diff.headers_diff[k]) + '</div></div>'
                ).join('');
            } else {
                html += '<div class="diff-none">No header changes</div>';
            }
            html += '</div>';

            // Body diff
            html += '<div class="diff-section"><h4>Body</h4>';
            if (diff.body_diff) {
                const lines = diff.body_diff.split('\n');
                const formatted = lines.map(l => {
                    if (l.startsWith('+')) return '<span class="diff-line-add">' + escapeHtml(l) + '</span>';
                    if (l.startsWith('-')) return '<span class="diff-line-remove">' + escapeHtml(l) + '</span>';
                    if (l.startsWith('~')) return '<span class="diff-line-change">' + escapeHtml(l) + '</span>';
                    return escapeHtml(l);
                }).join('\n');
                html += '<pre class="diff-body-pre">' + formatted + '</pre>';
            } else {
                html += '<div class="diff-none">Body unchanged</div>';
            }
            html += '</div>';

            el.innerHTML = html;
        } catch (e) {
            el.innerHTML = '<div class="diff-none">Error loading diff: ' + escapeHtml(String(e)) + '</div>';
        }
    }

    // ============================================================
    // Scenarios Feature Module
    // ============================================================
    let activeScenario = null;
    let recordingStatusTimer = null;
    let activeEditStepIndex = -1;

    async function fetchScenarios() {
        const listEl = $('#scenarios-list');
        if (!listEl) return;
        try {
            const res = await fetch('/api/scenarios');
            if (!res.ok) throw new Error('Failed to fetch scenarios');
            const scenarios = await res.json();

            if (!scenarios || scenarios.length === 0) {
                listEl.innerHTML = `
                    <div class="empty-state">
                        <p>No saved scenarios yet.</p>
                        <p class="empty-sub">Click "Start Recording", perform a real user flow in your app, then save as a named scenario.</p>
                    </div>`;
                return;
            }

            listEl.innerHTML = scenarios.map(sc => `
                <div class="scenario-card ${activeScenario && activeScenario.id === sc.id ? 'active' : ''}" data-id="${escapeHtml(sc.id)}">
                    <div class="scenario-card-title">${escapeHtml(sc.name)}</div>
                    <div class="scenario-card-desc">${escapeHtml(sc.description || 'Recorded traffic flow')}</div>
                    <div class="scenario-card-stats">
                        <span>${sc.request_count} requests</span>
                        <span>·</span>
                        <span>${sc.service_count} services</span>
                        <span>·</span>
                        <span>${(sc.total_delay_ms / 1000).toFixed(1)}s total delay</span>
                    </div>
                </div>
            `).join('');

            $$('.scenario-card').forEach(card => {
                card.addEventListener('click', () => {
                    selectScenario(card.dataset.id);
                });
            });

            // Check live recording status
            checkRecordingStatus();
        } catch (e) {
            listEl.innerHTML = `<div class="empty-state">Error loading scenarios: ${escapeHtml(e.message)}</div>`;
        }
    }

    async function selectScenario(id) {
        try {
            const res = await fetch(`/api/scenarios/${id}`);
            if (!res.ok) throw new Error('Failed to load scenario');
            activeScenario = await res.json();
            renderScenarioDetail(activeScenario);
            fetchScenarios(); // update active card highlight
        } catch (e) {
            showToast(e.message, 'error');
        }
    }

    function renderScenarioDetail(sc) {
        const emptyEl = $('#scenario-detail-empty');
        const viewEl = $('#scenario-detail-view');
        if (!emptyEl || !viewEl) return;

        emptyEl.classList.add('hidden');
        viewEl.classList.remove('hidden');

        $('#sc-detail-title').textContent = sc.name;
        
        let servicesCount = new Set();
        let totalDelay = 0;
        (sc.steps || []).forEach(step => {
            if (step.target_service) servicesCount.add(step.target_service);
            totalDelay += (step.delay_ms || 0);
        });

        $('#sc-detail-meta').textContent = `${(sc.steps || []).length} requests · ${servicesCount.size || 1} services · ${(totalDelay / 1000).toFixed(1)}s delay preservation`;

        // Render Variables
        renderScenarioVars(sc.variables || {});

        // Render Flow Graph
        renderFlowGraph(sc.steps || []);

        // Render Steps List
        renderStepsList(sc.steps || []);

        // Bind Actions
        const btnReplay = $('#btn-replay-scenario-active');
        if (btnReplay) {
            btnReplay.onclick = () => replayScenario(sc.id);
        }
        const btnDelete = $('#btn-delete-scenario-active');
        if (btnDelete) {
            btnDelete.onclick = async () => {
                if (!confirm(`Delete scenario "${sc.name}"?`)) return;
                try {
                    await fetch(`/api/scenarios/${sc.id}`, { method: 'DELETE' });
                    showToast('Scenario deleted', 'info');
                    activeScenario = null;
                    viewEl.classList.add('hidden');
                    emptyEl.classList.remove('hidden');
                    fetchScenarios();
                } catch (err) {
                    showToast('Delete failed', 'error');
                }
            };
        }
    }

    function renderScenarioVars(vars) {
        const list = $('#sc-vars-list');
        if (!list) return;

        const entries = Object.entries(vars);
        if (entries.length === 0) {
            list.innerHTML = `<span style="font-size:11px;color:var(--text-tertiary);font-style:italic;">No environment variables defined (Click + Add Variable to set BASE_URL, USER_ID, etc.)</span>`;
        } else {
            list.innerHTML = entries.map(([k, v]) => `
                <div class="var-pill">
                    <span class="var-key">${escapeHtml(k)}:</span>
                    <span class="var-val">${escapeHtml(v)}</span>
                    <button class="btn-icon btn-del-var" data-key="${escapeHtml(k)}" style="margin-left:4px;">&times;</button>
                </div>
            `).join('');

            $$('.btn-del-var').forEach(btn => {
                btn.onclick = (e) => {
                    e.stopPropagation();
                    delete activeScenario.variables[btn.dataset.key];
                    saveCurrentScenario();
                };
            });
        }

        const btnAdd = $('#btn-add-sc-var');
        if (btnAdd) {
            btnAdd.onclick = () => {
                const key = prompt('Variable Name (e.g. USER_ID or BASE_URL):');
                if (!key) return;
                const val = prompt(`Value for ${key}:`);
                if (val === null) return;
                if (!activeScenario.variables) activeScenario.variables = {};
                activeScenario.variables[key.trim()] = val.trim();
                saveCurrentScenario();
            };
        }
    }

    function renderFlowGraph(steps) {
        const graphEl = $('#sc-flow-graph');
        if (!graphEl) return;

        if (steps.length === 0) {
            graphEl.innerHTML = '<div class="empty-state">No steps in flow sequence</div>';
            return;
        }

        let html = `
            <div class="flow-node start-node">
                <div>START</div>
            </div>
        `;

        steps.forEach((step, idx) => {
            const delayStr = step.delay_ms ? `${step.delay_ms}ms` : '0ms';
            html += `
                <div class="flow-arrow">
                    <span>↓</span>
                    <span>${delayStr}</span>
                </div>
                <div class="flow-node">
                    <span class="flow-method method-${(step.method || 'GET').toLowerCase()}">${escapeHtml(step.method)}</span>
                    <span class="flow-path">${escapeHtml(step.path)}</span>
                    <span class="flow-meta">${step.expected_status ? step.expected_status + ' OK' : '200 OK'}</span>
                </div>
            `;
        });

        html += `
            <div class="flow-arrow">
                <span>↓</span>
            </div>
            <div class="flow-node end-node">
                <div>END</div>
            </div>
        `;

        graphEl.innerHTML = html;
    }

    function renderStepsList(steps) {
        const listEl = $('#sc-steps-list');
        if (!listEl) return;

        if (steps.length === 0) {
            listEl.innerHTML = '<div class="empty-state">No steps added</div>';
            return;
        }

        listEl.innerHTML = steps.map((step, idx) => `
            <div class="replay-step-row" style="margin-bottom:8px;">
                <div class="replay-step-header">
                    <div>
                        <strong style="margin-right:8px;">${idx + 1}. ${escapeHtml(step.name || step.method + ' ' + step.path)}</strong>
                        <span class="method-badge method-${(step.method || 'GET').toLowerCase()}">${escapeHtml(step.method)}</span>
                        <span style="font-family:var(--font-mono);font-size:11px;margin-left:6px;color:var(--text-main);">${escapeHtml(step.path)}</span>
                    </div>
                    <div style="display:flex;gap:6px;align-items:center;">
                        <span style="font-size:10px;color:var(--text-tertiary);margin-right:6px;">Expected: ${step.expected_status || 200}</span>
                        <button class="btn btn-xs btn-ghost btn-edit-step" data-idx="${idx}">Edit Step</button>
                        <button class="btn btn-xs btn-ghost btn-del-step" data-idx="${idx}">&times;</button>
                    </div>
                </div>
                ${step.extractions && step.extractions.length > 0 ? `
                    <div style="font-size:10px;color:var(--accent-primary);margin-top:4px;">
                        ⚡ Dynamic Extractions: ${step.extractions.map(e => `${e.var_name} = response.${e.expression}`).join(', ')}
                    </div>` : ''}
            </div>
        `).join('');

        $$('.btn-edit-step').forEach(btn => {
            btn.onclick = () => openEditStepModal(parseInt(btn.dataset.idx, 10));
        });

        $$('.btn-del-step').forEach(btn => {
            btn.onclick = () => {
                const idx = parseInt(btn.dataset.idx, 10);
                activeScenario.steps.splice(idx, 1);
                saveCurrentScenario();
            };
        });
    }

    async function saveCurrentScenario() {
        if (!activeScenario) return;
        try {
            const res = await fetch(`/api/scenarios/${activeScenario.id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(activeScenario)
            });
            if (!res.ok) throw new Error('Failed to save scenario');
            activeScenario = await res.json();
            renderScenarioDetail(activeScenario);
            showToast('Scenario saved', 'info');
        } catch (e) {
            showToast(e.message, 'error');
        }
    }

    // --- Recording Controls ---
    async function checkRecordingStatus() {
        try {
            const res = await fetch('/api/scenarios/record/status');
            if (!res.ok) return;
            const status = await res.json();
            updateRecordingUI(status);
        } catch (e) {}
    }

    function updateRecordingUI(status) {
        const banner = $('#scenario-rec-banner');
        const stats = $('#rec-banner-stats');
        const recBtnText = $('#rec-btn-text');

        if (status && status.is_recording) {
            if (banner) banner.classList.remove('hidden');
            if (stats) stats.textContent = `Scenario: "${status.scenario_name}" · Captured: ${status.request_count} requests · ${status.duration_sec.toFixed(1)}s`;
            if (recBtnText) recBtnText.textContent = `● Recording (${status.request_count})`;

            if (!recordingStatusTimer) {
                recordingStatusTimer = setInterval(checkRecordingStatus, 1500);
            }
        } else {
            if (banner) banner.classList.add('hidden');
            if (recBtnText) recBtnText.textContent = 'Record Scenario';
            if (recordingStatusTimer) {
                clearInterval(recordingStatusTimer);
                recordingStatusTimer = null;
            }
        }
    }

    async function startRecordingScenario() {
        const name = prompt('Scenario Name (e.g. checkout-flow):', 'checkout-flow');
        if (!name) return;
        try {
            const res = await fetch('/api/scenarios/record/start', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name })
            });
            if (!res.ok) throw new Error('Failed to start recording');
            const status = await res.json();
            updateRecordingUI(status);
            showToast(`Started traffic recording for "${name}". Perform actions in your app!`, 'info');
        } catch (e) {
            showToast(e.message, 'error');
        }
    }

    async function stopRecordingScenario() {
        const modal = $('#save-scenario-modal-overlay');
        if (modal) modal.classList.remove('hidden');
    }

    // --- Replay Execution Handler ---
    async function replayScenario(scName) {
        const modal = $('#replay-modal-overlay');
        const titleEl = $('#replay-modal-title');
        const summaryEl = $('#replay-status-summary');
        const resultsEl = $('#replay-step-results');

        if (!modal || !summaryEl || !resultsEl) return;

        modal.classList.remove('hidden');
        titleEl.textContent = `Replaying ${scName}...`;
        summaryEl.className = 'scenario-replay-summary';
        summaryEl.innerHTML = `<span class="status-indicator">Executing scenario steps sequentially...</span>`;
        resultsEl.innerHTML = `<div style="padding:20px;text-align:center;color:var(--text-tertiary);">Replaying requests with delay timing &amp; dynamic variable substitution...</div>`;

        try {
            const res = await fetch(`/api/scenarios/${scName}/replay`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ maintain_delay: true })
            });

            if (!res.ok) throw new Error('Replay server error');
            const result = await res.json();

            // Render Replay Outcome
            if (result.passed) {
                summaryEl.className = 'scenario-replay-summary passed';
                summaryEl.innerHTML = `✓ Scenario Passed — All ${result.passed_steps} / ${result.total_steps} requests successful (${(result.total_duration_ms / 1000).toFixed(2)}s)`;
            } else {
                summaryEl.className = 'scenario-replay-summary failed';
                summaryEl.innerHTML = `✗ Scenario Failed — ${result.passed_steps} / ${result.total_steps} requests successful (${result.failed_steps} failed)`;
            }

            resultsEl.innerHTML = (result.step_results || []).map(step => {
                const isPass = step.passed;
                const statusClass = getStatusClass(step.status_code);
                return `
                    <div class="replay-step-row ${isPass ? 'pass' : 'fail'}">
                        <div class="replay-step-header">
                            <div>
                                <span class="${isPass ? 'pass-badge' : 'fail-badge'}">${isPass ? '✓' : '✗'}</span>
                                <strong>${escapeHtml(step.step_name)}</strong>
                                <span class="method-badge method-${(step.method || 'GET').toLowerCase()}" style="margin-left:6px;">${escapeHtml(step.method)}</span>
                                <span style="font-family:var(--font-mono);font-size:11px;margin-left:6px;color:var(--text-main);">${escapeHtml(step.path)}</span>
                            </div>
                            <div>
                                <span class="status-badge ${statusClass}">${step.status_code}</span>
                                <span style="font-size:10px;color:var(--text-tertiary);margin-left:6px;">${step.duration_ms}ms</span>
                            </div>
                        </div>
                        ${(step.assertion_results || []).map(as => `
                            <div style="font-size:11px;margin-top:4px;color:${as.passed ? 'var(--success)' : 'var(--error)'};">
                                ${as.passed ? '✓ Assertion Pass' : '✗ Assertion Fail'}: Expected ${escapeHtml(as.expected)}, Received ${escapeHtml(as.received)} ${as.error ? '(' + escapeHtml(as.error) + ')' : ''}
                            </div>
                        `).join('')}
                        ${Object.keys(step.extracted_vars || {}).length > 0 ? `
                            <div style="font-size:10px;color:var(--accent-primary);margin-top:4px;">
                                ⚡ Extracted: ${Object.entries(step.extracted_vars).map(([k,v]) => `${k}="${v}"`).join(', ')}
                            </div>
                        ` : ''}
                        ${step.error ? `<div style="font-size:11px;color:var(--error);margin-top:4px;">Error: ${escapeHtml(step.error)}</div>` : ''}
                    </div>
                `;
            }).join('');

            // Also reload inspector requests list since replayed entries show up in live inspector!
            fetchRequests();

        } catch (e) {
            summaryEl.className = 'scenario-replay-summary failed';
            summaryEl.textContent = `Replay Execution Error: ${e.message}`;
        }
    }

    function openEditStepModal(idx) {
        if (!activeScenario || !activeScenario.steps[idx]) return;
        activeEditStepIndex = idx;
        const step = activeScenario.steps[idx];

        $('#edit-step-method').value = step.method || 'GET';
        $('#edit-step-path').value = step.path || '';
        $('#edit-step-status').value = step.expected_status || 200;
        $('#edit-step-delay').value = step.delay_ms || 200;
        $('#edit-step-body').value = step.body || '';

        const modal = $('#edit-step-modal-overlay');
        if (modal) modal.classList.remove('hidden');
    }

    // Modal listeners initialization for Scenarios
    function initScenarioModalEvents() {
        const btnRec = $('#btn-record-scenario');
        if (btnRec) btnRec.onclick = startRecordingScenario;

        const btnStartRec = $('#btn-start-scenario-rec');
        if (btnStartRec) btnStartRec.onclick = startRecordingScenario;

        const btnStopRec = $('#btn-stop-rec-scenario');
        if (btnStopRec) btnStopRec.onclick = stopRecordingScenario;

        // Save Scenario Modal
        const saveModal = $('#save-scenario-modal-overlay');
        const btnSaveClose = $('#save-sc-close');
        const btnSaveCancel = $('#btn-save-sc-cancel');
        const btnSaveConfirm = $('#btn-save-sc-confirm');

        if (btnSaveClose) btnSaveClose.onclick = () => saveModal.classList.add('hidden');
        if (btnSaveCancel) btnSaveCancel.onclick = () => saveModal.classList.add('hidden');
        if (btnSaveConfirm) {
            btnSaveConfirm.onclick = async () => {
                const name = $('#save-sc-name').value || 'checkout-flow';
                const desc = $('#save-sc-desc').value || 'Recorded traffic flow';
                try {
                    const res = await fetch('/api/scenarios/record/stop', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ name, description: desc })
                    });
                    if (!res.ok) throw new Error('Failed to save scenario');
                    const sc = await res.json();
                    saveModal.classList.add('hidden');
                    showToast(`Scenario "${sc.name}" saved!`, 'info');
                    checkRecordingStatus();
                    fetchScenarios();
                    selectScenario(sc.id);
                } catch (e) {
                    showToast(e.message, 'error');
                }
            };
        }

        // Edit Step Modal
        const editStepModal = $('#edit-step-modal-overlay');
        const btnEditStepClose = $('#edit-step-close');
        const btnEditStepCancel = $('#btn-edit-step-cancel');
        const btnEditStepSave = $('#btn-edit-step-save');

        if (btnEditStepClose) btnEditStepClose.onclick = () => editStepModal.classList.add('hidden');
        if (btnEditStepCancel) btnEditStepCancel.onclick = () => editStepModal.classList.add('hidden');
        if (btnEditStepSave) {
            btnEditStepSave.onclick = () => {
                if (activeEditStepIndex < 0 || !activeScenario || !activeScenario.steps[activeEditStepIndex]) return;
                const step = activeScenario.steps[activeEditStepIndex];
                step.method = $('#edit-step-method').value;
                step.path = $('#edit-step-path').value;
                step.expected_status = parseInt($('#edit-step-status').value, 10) || 200;
                step.delay_ms = parseInt($('#edit-step-delay').value, 10) || 0;
                step.body = $('#edit-step-body').value;

                editStepModal.classList.add('hidden');
                saveCurrentScenario();
            };
        }

        // Replay Modal Close
        const replayModal = $('#replay-modal-overlay');
        const btnReplayClose = $('#replay-modal-close');
        const btnReplayDone = $('#btn-replay-close-done');
        if (btnReplayClose) btnReplayClose.onclick = () => replayModal.classList.add('hidden');
        if (btnReplayDone) btnReplayDone.onclick = () => replayModal.classList.add('hidden');
    }

    // Call initScenarioModalEvents on init
    document.addEventListener('DOMContentLoaded', () => {
        setTimeout(initScenarioModalEvents, 100);
    });

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
        if (typeof body === 'string') {
            try { return atob(body); } catch (e) { return body; }
        }
        if (Array.isArray(body)) {
            try { return new TextDecoder().decode(new Uint8Array(body)); } catch (e) { return String(body); }
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
        if (!container) return;
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
