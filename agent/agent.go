package agent

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/middleware"
	"github.com/7uyash/routa/protocol"
	"github.com/7uyash/routa/proxy"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/replay"
	"github.com/7uyash/routa/router"
	"github.com/7uyash/routa/shadow"
	"github.com/7uyash/routa/storage"
	"github.com/7uyash/routa/tunnel"
	"github.com/7uyash/routa/webhook"
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

	mutator   *middleware.Mutator
	simulator *middleware.Simulator
	shadower  *shadow.Shadower

	mu sync.RWMutex
}

// New creates a new Agent with the given configuration.
func New(cfg config.Config) *Agent {
	rec := recorder.New(cfg.MaxRecordedEntries)
	fwd := proxy.New()

	// Setup router
	rtr := router.NewSingle(cfg.LocalTarget())
	if cfg.ProjectCfg != nil && len(cfg.ProjectCfg.Routes) > 0 {
		var routes []router.Route
		for _, r := range cfg.ProjectCfg.Routes {
			routes = append(routes, router.Route{
				Pattern: r.Pattern,
				Target:  r.Target,
				Name:    r.Name,
			})
		}
		rtr.SetRoutes(routes)
	}

	store := storage.NewStore(cfg.SessionsDir())
	wh := webhook.NewLab()
	rep := replay.New(fwd, rec)

	var mutRules []config.MutationConfig
	var simRules []config.SimulationConfig
	var shadCfg config.ShadowConfig
	if cfg.ProjectCfg != nil {
		mutRules = cfg.ProjectCfg.Mutations
		simRules = cfg.ProjectCfg.Simulations
		shadCfg = cfg.ProjectCfg.Shadow
	}

	a := &Agent{
		cfg:       cfg,
		proxy:     fwd,
		recorder:  rec,
		replay:    rep,
		router:    rtr,
		storage:   store,
		webhook:   wh,
		mutator:   middleware.NewMutator(mutRules),
		simulator: middleware.NewSimulator(simRules),
		shadower:  shadow.New(shadCfg),
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
	// We pass mutator/simulator/shadower into it later if we need to modify them via API.
	a.dashboard = NewDashboardServer(cfg.DashboardPort, rec, rep, store, wh, a.tunnel, cfg)
	a.dashboard.agent = a // Link back to agent to update routes/mutations

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
	log.Printf("[agent] forwarding to default target %s", a.cfg.LocalTarget())
	if a.shadower.TargetCount() > 0 {
		log.Printf("[agent] shadowing to %d target(s)", a.shadower.TargetCount())
	}

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

	// 1. Webhook check
	if epID := a.webhook.MatchPath(reqMsg.Path); epID != "" {
		a.webhook.RecordDelivery(epID, reqMsg.Method, reqMsg.Headers, reqMsg.Body)
	}

	// 2. Traffic Mutation (Request Phase)
	mutReq := a.mutator.ApplyToRequest(reqMsg.Method, reqMsg.Path, reqMsg.Query, reqMsg.Headers, reqMsg.Body)

	// Build base recorder entry
	entry := &recorder.Entry{
		Timestamp:      start,
		Method:         mutReq.Method,
		Path:           mutReq.Path,
		Query:          mutReq.Query,
		RequestHeaders: mutReq.Headers,
		RequestBody:    mutReq.Body,
		Host:           reqMsg.Host,
		Source:         "tunnel",
	}

	// If mock response is injected, skip everything else
	if mutReq.MockResponse != nil {
		entry.StatusCode = mutReq.MockResponse.Status
		hdr := make(map[string][]string)
		for k, v := range mutReq.MockResponse.Headers {
			hdr[k] = []string{v}
		}
		entry.ResponseHeaders = hdr
		entry.ResponseBody = mutReq.MockResponse.Body
		entry.Duration = time.Since(start)

		a.recorder.Record(entry)
		a.sendHTTPResponse(frame.RequestID, entry.StatusCode, hdr, entry.ResponseBody)
		return
	}

	// 3. Network Simulation
	simRes := a.simulator.Simulate(mutReq.Method, mutReq.Path)
	if simRes.ShouldDrop {
		// Log drop but don't record or respond
		log.Printf("[agent] dropping request to %s (rule: %s)", mutReq.Path, simRes.MatchedRule)
		return
	}
	if simRes.InjectedStatus != 0 {
		entry.StatusCode = simRes.InjectedStatus
		entry.Duration = time.Since(start)
		a.recorder.Record(entry)
		a.sendErrorResponse(frame.RequestID, simRes.InjectedStatus, fmt.Sprintf("Injected error (rule: %s)", simRes.MatchedRule))
		return
	}
	if simRes.Delay > 0 {
		middleware.ApplyDelay(simRes)
	}

	// 4. Routing
	target := a.router.Match(mutReq.Path)
	if target == "" {
		target = a.cfg.LocalTarget()
	}
	targetURL := target + mutReq.Path
	if mutReq.Query != "" {
		targetURL += "?" + mutReq.Query
	}
	entry.FullURL = targetURL

	// 5. Shadow Traffic
	if a.shadower.TargetCount() > 0 {
		go a.shadower.Shadow(entry, mutReq.Method, mutReq.Path, mutReq.Query, mutReq.Headers, mutReq.Body)
	}

	// 6. Forward to primary target
	resp, err := a.proxy.Forward(mutReq.Method, targetURL, mutReq.Headers, mutReq.Body)

	var outStatus int
	var outHeaders map[string][]string
	var outBody []byte

	if err != nil {
		outStatus = 502
		outHeaders = map[string][]string{"Content-Type": {"text/plain"}}
		outBody = []byte(fmt.Sprintf("Routa: failed to reach local service: %v", err))
		entry.Error = err.Error()
	} else {
		outStatus = resp.StatusCode
		outHeaders = resp.Headers
		outBody = resp.Body
		entry.TimingBreakdown = resp.Timing
	}

	// 7. Traffic Mutation (Response Phase)
	mutStatus, mutHeaders, mutBody := a.mutator.ApplyToResponse(mutReq.Method, mutReq.Path, outStatus, outHeaders, outBody)

	entry.StatusCode = mutStatus
	entry.ResponseHeaders = mutHeaders
	entry.ResponseBody = mutBody
	entry.Duration = time.Since(start)

	// Record and log
	a.recorder.Record(entry)

	parsedPath := mutReq.Path
	if u, err := url.Parse(reqMsg.URL); err == nil && u.Path != "" {
		parsedPath = u.Path
	}
	log.Printf("[agent] %s %s → %d (%s)",
		mutReq.Method, parsedPath, entry.StatusCode, entry.Duration.Round(time.Millisecond))

	// Send back to tunnel
	a.sendHTTPResponse(frame.RequestID, mutStatus, mutHeaders, mutBody)
}

func (a *Agent) sendHTTPResponse(requestID uint32, status int, headers map[string][]string, body []byte) {
	respMsg := protocol.HTTPResponseMsg{
		StatusCode: status,
		Headers:    headers,
		Body:       body,
	}
	respFrame, err := protocol.EncodePayload(protocol.TypeHTTPResponse, requestID, respMsg)
	if err != nil {
		log.Printf("[agent] encode response: %v", err)
		return
	}
	if err := a.tunnel.SendFrame(respFrame); err != nil {
		log.Printf("[agent] send response: %v", err)
	}
}

func (a *Agent) sendErrorResponse(requestID uint32, status int, message string) {
	a.sendHTTPResponse(requestID, status, map[string][]string{"Content-Type": {"text/plain"}}, []byte(message))
}
