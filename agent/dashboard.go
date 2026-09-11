package agent

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/diff"
	"github.com/7uyash/routa/discovery"
	"github.com/7uyash/routa/mock"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/replay"
	"github.com/7uyash/routa/router"
	"github.com/7uyash/routa/storage"
	"github.com/7uyash/routa/tunnel"
	"github.com/7uyash/routa/webhook"

	"github.com/gorilla/websocket"
)

//go:embed dashboard/static
var staticFiles embed.FS

// DashboardServer serves the web inspector dashboard and REST API.
type DashboardServer struct {
	port     int
	rec      *recorder.Recorder
	replay   *replay.Engine
	storage  *storage.Store
	webhook  *webhook.Lab
	scanner  *discovery.Scanner
	apiMap   *discovery.MapBuilder
	mockLab  *mock.Lab
	tunnel   *tunnel.Client
	cfg      config.Config
	server   *http.Server
	upgrader websocket.Upgrader
	agent    *Agent // back-pointer for hot-reloading routes/mutations

	// WebSocket clients for live updates.
	wsMu    sync.RWMutex
	wsConns map[*websocket.Conn]bool
}

// NewDashboardServer creates a dashboard server.
func NewDashboardServer(port int, rec *recorder.Recorder, rep *replay.Engine,
	store *storage.Store, wh *webhook.Lab, scn *discovery.Scanner, apiMap *discovery.MapBuilder,
	ml *mock.Lab, tun *tunnel.Client, cfg config.Config) *DashboardServer {

	ds := &DashboardServer{
		port:    port,
		rec:     rec,
		replay:  rep,
		storage: store,
		webhook: wh,
		scanner: scn,
		apiMap:  apiMap,
		mockLab: ml,
		tunnel:  tun,
		cfg:     cfg,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		wsConns: make(map[*websocket.Conn]bool),
	}

	// Register live update callback on the recorder.
	rec.OnChange(func(entry *recorder.Entry) {
		ds.broadcastEntry(entry)
	})

	return ds
}

// Start starts the dashboard HTTP server.
func (ds *DashboardServer) Start() error {
	mux := http.NewServeMux()

	// Phase 1 API routes.
	mux.HandleFunc("/api/requests", ds.handleRequests)
	mux.HandleFunc("/api/requests/", ds.handleRequestDetail)
	mux.HandleFunc("/api/replay", ds.handleEditReplay)
	mux.HandleFunc("/api/tunnel/status", ds.handleTunnelStatus)
	mux.HandleFunc("/api/sessions", ds.handleSessions)
	mux.HandleFunc("/api/sessions/", ds.handleSessionDetail)
	mux.HandleFunc("/api/webhooks", ds.handleWebhooks)
	mux.HandleFunc("/api/webhooks/", ds.handleWebhookDetail)
	mux.HandleFunc("/api/ws", ds.handleWebSocket)

	// Phase 2 API routes.
	mux.HandleFunc("/api/routes", ds.handleRoutes)
	mux.HandleFunc("/api/mutations", ds.handleMutations)
	mux.HandleFunc("/api/simulations", ds.handleSimulations)
	mux.HandleFunc("/api/shadow", ds.handleShadow)

	// Phase 3 API routes: Discovery, API Map, Mock Lab, Connect Anything.
	mux.HandleFunc("/api/discovery/services", ds.handleDiscoveryServices)
	mux.HandleFunc("/api/discovery/proposals", ds.handleDiscoveryProposals)
	mux.HandleFunc("/api/discovery/map", ds.handleAPIMap)
	mux.HandleFunc("/api/mocks", ds.handleMocks)
	mux.HandleFunc("/api/mocks/", ds.handleMockDetail)

	staticFS, err := fs.Sub(staticFiles, "dashboard/static")
	if err != nil {
		return fmt.Errorf("embed static files: %w", err)
	}

	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/" || p == "/index.html" || p == "/style.css" || p == "/app.js" || p == "/logo.png" || p == "/favicon.ico" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Forward arbitrary API traffic through the Agent local proxy & inspection engine
		if ds.agent != nil {
			ds.agent.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	})

	ds.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", ds.port),
		Handler: mux,
	}

	if err := ds.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Stop shuts down the dashboard server.
func (ds *DashboardServer) Stop() {
	if ds.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ds.server.Shutdown(ctx)
	}
}

// --- Request Handlers ---

func (ds *DashboardServer) handleRequests(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		filter := recorder.Filter{
			Method: r.URL.Query().Get("method"),
			Path:   r.URL.Query().Get("path"),
			Search: r.URL.Query().Get("search"),
			Source: r.URL.Query().Get("source"),
		}
		if s := r.URL.Query().Get("status"); s != "" {
			if code, err := strconv.Atoi(s); err == nil {
				filter.StatusCode = code
			}
		}
		if s := r.URL.Query().Get("status_min"); s != "" {
			if code, err := strconv.Atoi(s); err == nil {
				filter.StatusMin = code
			}
		}
		if s := r.URL.Query().Get("status_max"); s != "" {
			if code, err := strconv.Atoi(s); err == nil {
				filter.StatusMax = code
			}
		}
		if l := r.URL.Query().Get("limit"); l != "" {
			if limit, err := strconv.Atoi(l); err == nil {
				filter.Limit = limit
			}
		}
		if o := r.URL.Query().Get("offset"); o != "" {
			if offset, err := strconv.Atoi(o); err == nil {
				filter.Offset = offset
			}
		}
		entries := ds.rec.List(filter)
		writeJSON(w, http.StatusOK, map[string]any{
			"entries": entries,
			"total":   ds.rec.Count(),
		})

	case "DELETE":
		ds.rec.Clear()
		writeJSON(w, http.StatusOK, map[string]any{"cleared": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ds *DashboardServer) handleRequestDetail(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	// Extract ID from path: /api/requests/{id} or /api/requests/{id}/replay
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/requests/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing request ID", http.StatusBadRequest)
		return
	}
	id := parts[0]

	// Check for /replay action.
	if len(parts) > 1 && parts[1] == "replay" && r.Method == "POST" {
		entry, err := ds.replay.Replay(id, ds.cfg.LocalTarget())
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, entry)
		return
	}

	// GET detail.
	if r.Method == "GET" {
		entry := ds.rec.Get(id)
		if entry == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, entry)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (ds *DashboardServer) handleEditReplay(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "failed to read body"})
		return
	}

	var req replay.EditRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}

	entry, err := ds.replay.EditAndReplay(req, ds.cfg.LocalTarget())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

func (ds *DashboardServer) handleTunnelStatus(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	stats := ds.tunnel.Stats()
	state := stats.State.String()
	if ds.cfg.RelayURL == "" {
		state = "no_relay"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"state":           state,
		"public_url":      stats.PublicURL,
		"subdomain":       stats.Subdomain,
		"request_count":   stats.RequestCount,
		"reconnect_count": stats.ReconnectCount,
		"connected_at":    stats.ConnectedAt,
		"local_target":    ds.cfg.LocalTarget(),
		"dashboard_port":  ds.cfg.DashboardPort,
	})
}

func (ds *DashboardServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		sessions, err := ds.storage.ListSessions()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})

	case "POST":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1024))
		var req struct {
			Name string `json:"name"`
		}
		json.Unmarshal(body, &req)
		if req.Name == "" {
			req.Name = fmt.Sprintf("session-%d", time.Now().Unix())
		}
		entries := ds.rec.All()
		if err := ds.storage.SaveSession(req.Name, entries); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"name":        req.Name,
			"entry_count": len(entries),
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ds *DashboardServer) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing session name", http.StatusBadRequest)
		return
	}
	name := parts[0]

	switch {
	case len(parts) > 1 && parts[1] == "load" && r.Method == "POST":
		session, err := ds.storage.LoadSession(name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			return
		}
		ds.rec.Clear()
		for _, entry := range session.Entries {
			ds.rec.Record(entry)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"name":   session.Name,
			"loaded": len(session.Entries),
		})

	case r.Method == "DELETE":
		if err := ds.storage.DeleteSession(name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ds *DashboardServer) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		endpoints := ds.webhook.ListEndpoints()
		writeJSON(w, http.StatusOK, map[string]any{"endpoints": endpoints})

	case "POST":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 2048))
		var req struct {
			Name     string `json:"name"`
			Provider string `json:"provider"`
			Secret   string `json:"secret"`
		}
		json.Unmarshal(body, &req)
		if req.Name == "" {
			req.Name = "webhook"
		}
		ep := ds.webhook.CreateEndpoint(req.Name, req.Provider, req.Secret)
		writeJSON(w, http.StatusCreated, ep)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ds *DashboardServer) handleWebhookDetail(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/webhooks/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing webhook ID", http.StatusBadRequest)
		return
	}
	id := parts[0]

	// /api/webhooks/:id/toggle
	if len(parts) > 1 && parts[1] == "toggle" && r.Method == "POST" {
		active, ok := ds.webhook.ToggleEndpoint(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "active": active})
		return
	}

	// /api/webhooks/:id/test
	if len(parts) > 1 && parts[1] == "test" && r.Method == "POST" {
		delivery, ok := ds.webhook.SimulateTestDelivery(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "endpoint not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery": delivery, "status": "simulated"})
		return
	}

	switch r.Method {
	case "GET":
		deliveries := ds.webhook.GetDeliveries(id)
		ep := ds.webhook.GetEndpoint(id)
		if ep == nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"endpoint":   ep,
			"deliveries": deliveries,
		})

	case "DELETE":
		ds.webhook.DeleteEndpoint(id)
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (ds *DashboardServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ds.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[dashboard] ws upgrade: %v", err)
		return
	}

	ds.wsMu.Lock()
	ds.wsConns[conn] = true
	ds.wsMu.Unlock()

	// Keep reading to detect disconnect.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	ds.wsMu.Lock()
	delete(ds.wsConns, conn)
	ds.wsMu.Unlock()
	conn.Close()
}

// broadcastEntry pushes a new entry to all connected WebSocket clients.
func (ds *DashboardServer) broadcastEntry(entry *recorder.Entry) {
	data, err := json.Marshal(map[string]any{
		"type":  "new_request",
		"entry": entry.Summary(),
	})
	if err != nil {
		return
	}

	ds.wsMu.RLock()
	defer ds.wsMu.RUnlock()

	for conn := range ds.wsConns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			go func(c *websocket.Conn) {
				ds.wsMu.Lock()
				delete(ds.wsConns, c)
				ds.wsMu.Unlock()
			}(conn)
		}
	}
}

// --- Helpers ---

func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ============================================================
// Phase 2 Handlers
// ============================================================

// handleRoutes — GET: list routes, PUT: replace route table.
func (ds *DashboardServer) handleRoutes(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if ds.agent == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent not available"})
		return
	}

	switch r.Method {
	case "GET":
		routes := ds.agent.router.Routes()
		writeJSON(w, http.StatusOK, map[string]any{"routes": routes})

	case "PUT":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		var req struct {
			Routes []router.Route `json:"routes"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}
		ds.agent.router.SetRoutes(req.Routes)
		writeJSON(w, http.StatusOK, map[string]any{"routes": req.Routes})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMutations — GET: list rules, PUT: replace rules.
func (ds *DashboardServer) handleMutations(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if ds.agent == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent not available"})
		return
	}

	switch r.Method {
	case "GET":
		rules := ds.agent.mutator.Rules()
		writeJSON(w, http.StatusOK, map[string]any{"mutations": rules})

	case "PUT":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		var req struct {
			Mutations []config.MutationConfig `json:"mutations"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}
		ds.agent.mutator.SetRules(req.Mutations)
		writeJSON(w, http.StatusOK, map[string]any{"mutations": req.Mutations})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSimulations — GET: list rules, PUT: replace rules.
func (ds *DashboardServer) handleSimulations(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if ds.agent == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent not available"})
		return
	}

	switch r.Method {
	case "GET":
		rules := ds.agent.simulator.Rules()
		writeJSON(w, http.StatusOK, map[string]any{"simulations": rules})

	case "PUT":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		var req struct {
			Simulations []config.SimulationConfig `json:"simulations"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}
		ds.agent.simulator.SetRules(req.Simulations)
		writeJSON(w, http.StatusOK, map[string]any{"simulations": req.Simulations})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleShadow — GET: shadow config.
func (ds *DashboardServer) handleShadow(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if ds.agent == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent not available"})
		return
	}

	switch r.Method {
	case "GET":
		writeJSON(w, http.StatusOK, map[string]any{
			"target_count": ds.agent.shadower.TargetCount(),
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRequestDiff — GET diff between a replay entry and its original.
func (ds *DashboardServer) handleRequestDiff(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/requests/"), "/")
	if len(parts) < 2 {
		http.Error(w, "missing ID", http.StatusBadRequest)
		return
	}
	id := parts[0]

	entry := ds.rec.Get(id)
	if entry == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}

	if entry.OriginalID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "entry has no original to diff against"})
		return
	}

	original := ds.rec.Get(entry.OriginalID)
	if original == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "original entry not found"})
		return
	}

	result := diff.Compare(original, entry)
	writeJSON(w, http.StatusOK, result)
}

// handleSessionPlayback — POST /api/sessions/:name/playback
func (ds *DashboardServer) handleSessionPlayback(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing session name", http.StatusBadRequest)
		return
	}
	name := parts[0]

	body, _ := io.ReadAll(io.LimitReader(r.Body, 1024))
	var opts storage.PlaybackOptions
	json.Unmarshal(body, &opts)
	opts.SessionName = name
	if opts.Target == "" {
		opts.Target = ds.cfg.LocalTarget()
	}

	fwd := ds.agent.proxy
	pb := storage.NewPlayback(ds.storage, fwd, ds.rec)

	go func() {
		if err := pb.Play(r.Context(), opts); err != nil {
			log.Printf("[dashboard] playback error: %v", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":  "started",
		"session": name,
		"target":  opts.Target,
	})
}

// ============================================================
// Phase 3 Handlers: Service Discovery, API Map, Mock Lab
// ============================================================

// handleDiscoveryServices — GET: trigger scan & return discovered services.
func (ds *DashboardServer) handleDiscoveryServices(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		// Return cached services without re-scanning
		services := ds.scanner.GetServices()
		writeJSON(w, http.StatusOK, map[string]any{"services": services})

	case "POST":
		// Trigger a fresh scan and return updated services
		services := ds.scanner.Scan()
		writeJSON(w, http.StatusOK, map[string]any{"services": services, "status": "ok"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDiscoveryProposals — GET: list proposals; POST: confirm a proposal.
func (ds *DashboardServer) handleDiscoveryProposals(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		proposals := ds.scanner.GetProposals()
		writeJSON(w, http.StatusOK, map[string]any{"proposals": proposals})

	case "POST":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		var req struct {
			Action      string `json:"action"` // "propose" or "confirm"
			ID          string `json:"id"`
			PathPattern string `json:"path_pattern"`
			TargetURL   string `json:"target_url"`
			ServiceName string `json:"service_name"`
		}
		json.Unmarshal(body, &req)

		if req.Action == "confirm" {
			prop, ok := ds.scanner.ConfirmProposal(req.ID)
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "proposal not found"})
				return
			}
			// Apply the confirmed route into the router
			if ds.agent != nil {
				existing := ds.agent.router.Routes()
				existing = append(existing, router.Route{
					Pattern: prop.PathPattern,
					Target:  prop.TargetURL,
					Name:    prop.ServiceName,
				})
				ds.agent.router.SetRoutes(existing)
			}
			writeJSON(w, http.StatusOK, map[string]any{"proposal": prop, "route_applied": true})
			return
		}

		// Propose a new route
		if req.PathPattern == "" || req.TargetURL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "path_pattern and target_url required"})
			return
		}
		prop := ds.scanner.ProposeRoute(req.PathPattern, req.TargetURL, req.ServiceName)
		writeJSON(w, http.StatusCreated, prop)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAPIMap — GET: return normalized API endpoint map from recorded traffic.
func (ds *DashboardServer) handleAPIMap(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	entries := ds.rec.All()
	endpointMap := ds.apiMap.BuildMap(entries)
	writeJSON(w, http.StatusOK, map[string]any{
		"endpoints": endpointMap,
		"total":     len(endpointMap),
	})
}

// handleMocks — GET: list mocks; POST: create mock (manual or from-request).
func (ds *DashboardServer) handleMocks(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		rules := ds.mockLab.ListRules()
		writeJSON(w, http.StatusOK, map[string]any{"mocks": rules})

	case "POST":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		var req struct {
			FromRequestID string            `json:"from_request_id"` // 1-click: convert captured entry
			Name          string            `json:"name"`
			Method        string            `json:"method"`
			Path          string            `json:"path"`
			Status        int               `json:"status"`
			Headers       map[string]string `json:"headers"`
			Body          string            `json:"body"`
			DelayMs       int               `json:"delay_ms"`
			Active        bool              `json:"active"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}

		// 1-click traffic-to-mock flow
		if req.FromRequestID != "" {
			entry := ds.rec.Get(req.FromRequestID)
			if entry == nil {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "request entry not found"})
				return
			}
			rule := ds.mockLab.CreateFromRequest(entry)
			writeJSON(w, http.StatusCreated, rule)
			return
		}

		// Manual mock creation
		if req.Method == "" {
			req.Method = "GET"
		}
		if req.Status == 0 {
			req.Status = 200
		}
		rule := ds.mockLab.CreateRule(req.Name, req.Method, req.Path, req.Status, req.Headers, req.Body, req.DelayMs, req.Active)
		writeJSON(w, http.StatusCreated, rule)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMockDetail — PUT: update mock; DELETE: remove mock.
func (ds *DashboardServer) handleMockDetail(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/mocks/")
	if id == "" {
		http.Error(w, "Missing mock ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "PUT":
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
		var req struct {
			Name    string            `json:"name"`
			Method  string            `json:"method"`
			Path    string            `json:"path"`
			Status  int               `json:"status"`
			Headers map[string]string `json:"headers"`
			Body    string            `json:"body"`
			DelayMs int               `json:"delay_ms"`
			Active  bool              `json:"active"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
			return
		}
		rule, ok := ds.mockLab.UpdateRule(id, req.Name, req.Method, req.Path, req.Status, req.Headers, req.Body, req.DelayMs, req.Active)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "mock not found"})
			return
		}
		writeJSON(w, http.StatusOK, rule)

	case "DELETE":
		if ok := ds.mockLab.DeleteRule(id); !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "mock not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Ensure unused imports don't cause build errors.
var _ = recorder.Entry{}
