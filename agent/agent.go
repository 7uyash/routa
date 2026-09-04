// Package agent implements the local Routa agent that manages the tunnel
// connection, proxies traffic, records requests, and serves the dashboard.
package agent

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"routa/config"
	"routa/protocol"
	"routa/proxy"
	"routa/recorder"
	"routa/replay"
	"routa/router"
	"routa/storage"
	"routa/tunnel"
	"routa/webhook"
)

// Agent is the main local component that orchestrates tunnel, proxy,
// recording, replay, and the dashboard.
type Agent struct {
	cfg       config.Config
	tunnel    *tunnel.Client
	proxy     *proxy.Forwarder
	recorder  *recorder.Recorder
	replay    *replay.Engine
	router    *router.Router
	storage   *storage.Store
	webhook   *webhook.Lab
	dashboard *DashboardServer

	mu sync.RWMutex
}

// New creates a new Agent with the given configuration.
func New(cfg config.Config) *Agent {
	rec := recorder.New(cfg.MaxRecordedEntries)
	fwd := proxy.New()
	rtr := router.NewSingle(cfg.LocalTarget())
	store := storage.NewStore(cfg.SessionsDir())
	wh := webhook.NewLab()
	rep := replay.New(fwd, rec)

	a := &Agent{
		cfg:      cfg,
		proxy:    fwd,
		recorder: rec,
		replay:   rep,
		router:   rtr,
		storage:  store,
		webhook:  wh,
	}

	// Create tunnel client.
	a.tunnel = tunnel.NewClient(cfg.RelayURL, cfg.AuthToken, cfg.TunnelName)
	a.tunnel.OnFrame = a.handleFrame
	a.tunnel.OnConnect = func() {
		log.Printf("[agent] tunnel connected: %s", a.tunnel.PublicURL)
	}
	a.tunnel.OnDisconnect = func() {
		log.Printf("[agent] tunnel disconnected, reconnecting...")
	}

	// Create dashboard server.
	a.dashboard = NewDashboardServer(cfg.DashboardPort, rec, rep, store, wh, a.tunnel, cfg)

	return a
}

// Start launches the agent: connects tunnel, starts dashboard, begins serving.
func (a *Agent) Start(ctx context.Context) error {
	// Start dashboard in background.
	go func() {
		if err := a.dashboard.Start(); err != nil {
			log.Printf("[agent] dashboard error: %v", err)
		}
	}()

	log.Printf("[agent] dashboard at http://localhost:%d", a.cfg.DashboardPort)
	log.Printf("[agent] forwarding to %s", a.cfg.LocalTarget())

	// Connect tunnel (blocks until stopped or fatal error).
	return a.tunnel.Connect(ctx)
}

// Stop gracefully shuts down the agent.
func (a *Agent) Stop() {
	a.tunnel.Stop()
	a.dashboard.Stop()
}

// PublicURL returns the assigned public URL (available after connection).
func (a *Agent) PublicURL() string {
	return a.tunnel.PublicURL
}

// TunnelStats returns current tunnel statistics.
func (a *Agent) TunnelStats() tunnel.Stats {
	return a.tunnel.Stats()
}

// RequestCount returns the number of recorded requests.
func (a *Agent) RequestCount() int {
	return a.recorder.Count()
}

// handleFrame processes incoming frames from the relay.
func (a *Agent) handleFrame(frame protocol.Frame) {
	switch frame.Type {
	case protocol.TypeHTTPRequest:
		go a.handleHTTPRequest(frame)
	default:
		log.Printf("[agent] unhandled frame type: %s", protocol.TypeName(frame.Type))
	}
}

// handleHTTPRequest proxies an HTTP request to the local service and sends
// the response back through the tunnel.
func (a *Agent) handleHTTPRequest(frame protocol.Frame) {
	var reqMsg protocol.HTTPRequestMsg
	if err := protocol.DecodePayload(frame, &reqMsg); err != nil {
		log.Printf("[agent] decode request: %v", err)
		a.sendErrorResponse(frame.RequestID, 400, "invalid request")
		return
	}

	start := time.Now()

	// Check if this is a webhook path.
	if epID := a.webhook.MatchPath(reqMsg.Path); epID != "" {
		a.webhook.RecordDelivery(epID, reqMsg.Method, reqMsg.Headers, reqMsg.Body)
	}

	// Route to local target.
	target := a.router.Match(reqMsg.Path)
	if target == "" {
		target = a.cfg.LocalTarget()
	}

	targetURL := target + reqMsg.Path
	if reqMsg.Query != "" {
		targetURL += "?" + reqMsg.Query
	}

	resp, err := a.proxy.Forward(reqMsg.Method, targetURL, reqMsg.Headers, reqMsg.Body)

	// Build recorder entry.
	entry := &recorder.Entry{
		Timestamp:      time.Now(),
		Method:         reqMsg.Method,
		Path:           reqMsg.Path,
		Query:          reqMsg.Query,
		RequestHeaders: reqMsg.Headers,
		RequestBody:    reqMsg.Body,
		Host:           reqMsg.Host,
		FullURL:        targetURL,
		Source:         "tunnel",
	}

	var respMsg protocol.HTTPResponseMsg

	if err != nil {
		log.Printf("[agent] forward error: %v", err)
		entry.StatusCode = 502
		entry.Error = err.Error()
		entry.Duration = time.Since(start)
		respMsg = protocol.HTTPResponseMsg{
			StatusCode: 502,
			Headers:    map[string][]string{"Content-Type": {"text/plain"}},
			Body:       []byte(fmt.Sprintf("Routa: failed to reach local service: %v", err)),
		}
	} else {
		entry.StatusCode = resp.StatusCode
		entry.ResponseHeaders = resp.Headers
		entry.ResponseBody = resp.Body
		entry.Duration = time.Since(start)
		entry.TimingBreakdown = resp.Timing
		respMsg = protocol.HTTPResponseMsg{
			StatusCode: resp.StatusCode,
			Headers:    resp.Headers,
			Body:       resp.Body,
		}
	}

	// Record the entry.
	a.recorder.Record(entry)

	// Parse URL for logging.
	parsedPath := reqMsg.Path
	if u, err := url.Parse(reqMsg.URL); err == nil && u.Path != "" {
		parsedPath = u.Path
	}
	log.Printf("[agent] %s %s → %d (%s)",
		reqMsg.Method, parsedPath, entry.StatusCode, entry.Duration.Round(time.Millisecond))

	// Send response back through tunnel.
	respFrame, err := protocol.EncodePayload(protocol.TypeHTTPResponse, frame.RequestID, respMsg)
	if err != nil {
		log.Printf("[agent] encode response: %v", err)
		return
	}
	if err := a.tunnel.SendFrame(respFrame); err != nil {
		log.Printf("[agent] send response: %v", err)
	}
}

// sendErrorResponse sends an error response frame back through the tunnel.
func (a *Agent) sendErrorResponse(requestID uint32, status int, message string) {
	respMsg := protocol.HTTPResponseMsg{
		StatusCode: status,
		Headers:    map[string][]string{"Content-Type": {"text/plain"}},
		Body:       []byte(message),
	}
	respFrame, _ := protocol.EncodePayload(protocol.TypeHTTPResponse, requestID, respMsg)
	var buf bytes.Buffer
	respFrame.Encode(&buf)
	a.tunnel.SendFrame(respFrame)
}
