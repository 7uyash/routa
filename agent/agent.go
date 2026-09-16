package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/discovery"
	"github.com/7uyash/routa/middleware"
	"github.com/7uyash/routa/mock"
	"github.com/7uyash/routa/protocol"
	"github.com/7uyash/routa/proxy"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/replay"
	"github.com/7uyash/routa/router"
	"github.com/7uyash/routa/shadow"
	"github.com/7uyash/routa/storage"
	"github.com/7uyash/routa/traffic"
	"github.com/7uyash/routa/tunnel"
	"github.com/7uyash/routa/webhook"
)

// Agent is the main local component that orchestrates tunnel, proxy,
// recording, replay, discovery, mock lab, and the dashboard.
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

	scanner   *discovery.Scanner
	apiMap    *discovery.MapBuilder
	mockLab   *mock.Lab

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

	scn := discovery.NewScanner()
	apiMap := discovery.NewMapBuilder()
	mockLab := mock.NewLab()

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
		scanner:   scn,
		apiMap:    apiMap,
		mockLab:   mockLab,
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
	a.dashboard = NewDashboardServer(cfg.DashboardPort, rec, rep, store, wh, scn, apiMap, mockLab, a.tunnel, cfg)
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

	// Initial discovery scan
	go func() {
		time.Sleep(500 * time.Millisecond)
		a.scanner.Scan()
	}()

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

	// Convert protocol edge-type → canonical traffic.Request for the internal pipeline.
	incomingReq := traffic.Request{
		Method:  reqMsg.Method,
		Path:    reqMsg.Path,
		Query:   reqMsg.Query,
		Headers: reqMsg.Headers,
		Body:    reqMsg.Body,
		Host:    reqMsg.Host,
	}

	// 1. Webhook check
	if epID := a.webhook.MatchPath(incomingReq.Path); epID != "" {
		a.webhook.RecordDelivery(epID, incomingReq.Method, incomingReq.Headers, incomingReq.Body)
	}

	// 2. Mock Lab match check (short-circuits backend forwarding if a mock rule matches)
	if mockRule := a.mockLab.MatchRequest(incomingReq.Method, incomingReq.Path); mockRule != nil {
		status, headers, body := mockRule.ServeMock()
		respHeaders := make(map[string][]string, len(headers))
		for k, v := range headers {
			respHeaders[k] = []string{v}
		}
		entry := &recorder.Entry{
			Timestamp:       start,
			Method:          incomingReq.Method,
			Path:            incomingReq.Path,
			Query:           incomingReq.Query,
			RequestHeaders:  incomingReq.Headers,
			RequestBody:     incomingReq.Body,
			StatusCode:      status,
			ResponseHeaders: respHeaders,
			ResponseBody:    body,
			Duration:        time.Since(start),
			Host:            incomingReq.Host,
			Source:          "mock_lab",
		}
		a.recorder.Record(entry)
		a.sendHTTPResponse(frame.RequestID, status, respHeaders, body)
		log.Printf("[agent] [Mock Lab] %s %s → %d (%s)", incomingReq.Method, incomingReq.Path, status, entry.Duration.Round(time.Millisecond))
		return
	}

	// 3. Traffic Mutation (Request Phase)
	mutResult := a.mutator.ApplyToRequest(incomingReq)
	mutReq := mutResult.Request

	// Build base recorder entry from the (possibly mutated) request.
	entry := &recorder.Entry{
		Timestamp:      start,
		Method:         mutReq.Method,
		Path:           mutReq.Path,
		Query:          mutReq.Query,
		RequestHeaders: mutReq.Headers,
		RequestBody:    mutReq.Body,
		Host:           mutReq.Host,
		Source:         "tunnel",
	}

	// If mutation injected a mock response, skip backend forwarding.
	if mutResult.MockResponse != nil {
		mock := mutResult.MockResponse
		entry.StatusCode = mock.StatusCode
		entry.ResponseHeaders = mock.Headers
		entry.ResponseBody = mock.Body
		entry.Duration = time.Since(start)
		a.recorder.Record(entry)
		a.sendHTTPResponse(frame.RequestID, mock.StatusCode, mock.Headers, mock.Body)
		return
	}

	// 4. Network Simulation
	simRes := a.simulator.Simulate(mutReq)
	if simRes.ShouldDrop {
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

	// 5. Routing
	target := a.router.Match(mutReq.Path)
	if target == "" {
		target = a.cfg.LocalTarget()
	}
	targetURL := target + mutReq.Path
	if mutReq.Query != "" {
		targetURL += "?" + mutReq.Query
	}
	entry.FullURL = targetURL

	// 6. Shadow Traffic
	if a.shadower.TargetCount() > 0 {
		go a.shadower.Shadow(entry, mutReq)
	}

	// 7. Forward to primary target
	resp, err := a.proxy.Forward(mutReq, targetURL)

	var proxyResp *traffic.Response
	if err != nil {
		// Synthesise an error response so the mutation phase still has a consistent type.
		proxyResp = &traffic.Response{
			StatusCode: 502,
			Headers:    map[string][]string{"Content-Type": {"text/plain"}},
			Body:       []byte(fmt.Sprintf("Routa: failed to reach local service: %v", err)),
		}
		entry.Error = err.Error()
	} else {
		proxyResp = resp
		entry.TimingBreakdown = resp.Timing
	}

	// 8. Traffic Mutation (Response Phase) — mutates proxyResp in-place.
	a.mutator.ApplyToResponse(mutReq, proxyResp)

	entry.StatusCode = proxyResp.StatusCode
	entry.ResponseHeaders = proxyResp.Headers
	entry.ResponseBody = proxyResp.Body
	entry.Duration = time.Since(start)

	// Record and log
	a.recorder.Record(entry)

	parsedPath := mutReq.Path
	if u, err := url.Parse(reqMsg.URL); err == nil && u.Path != "" {
		parsedPath = u.Path
	}
	log.Printf("[agent] %s %s → %d (%s)",
		mutReq.Method, parsedPath, entry.StatusCode, entry.Duration.Round(time.Millisecond))

	// Send back through the tunnel (convert traffic types back to protocol edge types).
	a.sendHTTPResponse(frame.RequestID, proxyResp.StatusCode, proxyResp.Headers, proxyResp.Body)
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

// ServeHTTP handles incoming local HTTP requests, passing them through
// the mock lab, mutator, simulator, router, and recorder pipeline.
func (a *Agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, _ := io.ReadAll(r.Body)

	path := r.URL.Path
	query := r.URL.RawQuery
	headers := make(map[string][]string)
	for k, v := range r.Header {
		headers[k] = v
	}

	// 1. Webhook check
	if epID := a.webhook.MatchPath(path); epID != "" {
		a.webhook.RecordDelivery(epID, r.Method, headers, body)
	}

	// 2. Mock Lab match check
	if mockRule := a.mockLab.MatchRequest(r.Method, path); mockRule != nil {
		status, mHeaders, mBody := mockRule.ServeMock()
		entry := &recorder.Entry{
			Timestamp:       start,
			Method:          r.Method,
			Path:            path,
			Query:           query,
			RequestHeaders:  headers,
			RequestBody:     body,
			StatusCode:      status,
			ResponseHeaders: make(map[string][]string),
			ResponseBody:    mBody,
			Duration:        time.Since(start),
			Host:            r.Host,
			Source:          "mock_lab",
		}
		for k, v := range mHeaders {
			entry.ResponseHeaders[k] = []string{v}
			w.Header().Set(k, v)
		}
		a.recorder.Record(entry)
		w.WriteHeader(status)
		w.Write(mBody)
		log.Printf("[agent] [Mock Lab] %s %s → %d (%s)", r.Method, path, status, entry.Duration.Round(time.Millisecond))
		return
	}

	// 3. Traffic Mutation (Request Phase)
	incomingLocalReq := traffic.Request{
		Method:  r.Method,
		Path:    path,
		Query:   query,
		Headers: headers,
		Body:    body,
		Host:    r.Host,
	}
	mutResult := a.mutator.ApplyToRequest(incomingLocalReq)
	mutReq := mutResult.Request

	entry := &recorder.Entry{
		Timestamp:      start,
		Method:         mutReq.Method,
		Path:           mutReq.Path,
		Query:          mutReq.Query,
		RequestHeaders: mutReq.Headers,
		RequestBody:    mutReq.Body,
		Host:           mutReq.Host,
		Source:         "local_proxy",
	}

	// If mutation injected a mock response, return immediately.
	if mutResult.MockResponse != nil {
		mock := mutResult.MockResponse
		entry.StatusCode = mock.StatusCode
		entry.ResponseHeaders = mock.Headers
		entry.ResponseBody = mock.Body
		entry.Duration = time.Since(start)
		for k, vals := range mock.Headers {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		a.recorder.Record(entry)
		w.WriteHeader(mock.StatusCode)
		w.Write(mock.Body)
		return
	}

	// 4. Network Simulation
	simRes := a.simulator.Simulate(mutReq)
	if simRes.ShouldDrop {
		log.Printf("[agent] dropping request to %s (rule: %s)", mutReq.Path, simRes.MatchedRule)
		return
	}
	if simRes.InjectedStatus != 0 {
		entry.StatusCode = simRes.InjectedStatus
		entry.Duration = time.Since(start)
		a.recorder.Record(entry)
		w.WriteHeader(simRes.InjectedStatus)
		w.Write([]byte(fmt.Sprintf("Injected error (rule: %s)", simRes.MatchedRule)))
		return
	}
	if simRes.Delay > 0 {
		middleware.ApplyDelay(simRes)
	}

	// 5. Routing
	target := a.router.Match(mutReq.Path)
	if target == "" {
		target = a.cfg.LocalTarget()
	}
	targetURL := target + mutReq.Path
	if mutReq.Query != "" {
		targetURL += "?" + mutReq.Query
	}
	entry.FullURL = targetURL

	// 6. Shadow Traffic
	if a.shadower.TargetCount() > 0 {
		go a.shadower.Shadow(entry, mutReq)
	}

	// 7. Forward to primary target
	resp, err := a.proxy.Forward(mutReq, targetURL)

	var proxyResp *traffic.Response
	if err != nil {
		proxyResp = &traffic.Response{
			StatusCode: 502,
			Headers:    map[string][]string{"Content-Type": {"text/plain"}},
			Body:       []byte(fmt.Sprintf("Routa: failed to reach local service: %v", err)),
		}
		entry.Error = err.Error()
	} else {
		proxyResp = resp
		entry.TimingBreakdown = resp.Timing
	}

	// 8. Traffic Mutation (Response Phase) — mutates proxyResp in-place.
	a.mutator.ApplyToResponse(mutReq, proxyResp)

	entry.StatusCode = proxyResp.StatusCode
	entry.ResponseHeaders = proxyResp.Headers
	entry.ResponseBody = proxyResp.Body
	entry.Duration = time.Since(start)

	a.recorder.Record(entry)

	for k, vals := range proxyResp.Headers {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(proxyResp.StatusCode)
	w.Write(proxyResp.Body)

	log.Printf("[agent] %s %s → %d (%s)", mutReq.Method, mutReq.Path, entry.StatusCode, entry.Duration.Round(time.Millisecond))
}

