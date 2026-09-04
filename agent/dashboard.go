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

	"routa/config"
	"routa/recorder"
	"routa/replay"
	"routa/storage"
	"routa/tunnel"
	"routa/webhook"

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
	store *storage.Store, wh *webhook.Lab, tun *tunnel.Client, cfg config.Config) *DashboardServer {

	ds := &DashboardServer{
		port:    port,
		rec:     rec,
		replay:  rep,
		storage: store,
		webhook: wh,
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

	// API routes.
	mux.HandleFunc("/api/requests", ds.handleRequests)
	mux.HandleFunc("/api/requests/", ds.handleRequestDetail)
	mux.HandleFunc("/api/replay", ds.handleEditReplay)
	mux.HandleFunc("/api/tunnel/status", ds.handleTunnelStatus)
	mux.HandleFunc("/api/sessions", ds.handleSessions)
	mux.HandleFunc("/api/sessions/", ds.handleSessionDetail)
	mux.HandleFunc("/api/webhooks", ds.handleWebhooks)
	mux.HandleFunc("/api/webhooks/", ds.handleWebhookDetail)
	mux.HandleFunc("/api/ws", ds.handleWebSocket)

	// Serve embedded static files.
	staticFS, err := fs.Sub(staticFiles, "dashboard/static")
	if err != nil {
		return fmt.Errorf("embed static files: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

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
	writeJSON(w, http.StatusOK, map[string]any{
		"state":           stats.State.String(),
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
			"name":    session.Name,
			"loaded":  len(session.Entries),
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
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1024))
		var req struct {
			Name string `json:"name"`
		}
		json.Unmarshal(body, &req)
		if req.Name == "" {
			req.Name = "webhook"
		}
		ep := ds.webhook.CreateEndpoint(req.Name)
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
