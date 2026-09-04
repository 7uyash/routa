package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/7uyash/routa/protocol"
	"github.com/7uyash/routa/tunnel"

	"github.com/gorilla/websocket"
)

// Server is the public relay edge server.
type Server struct {
	registry   *TunnelRegistry
	baseDomain string
	upgrader   websocket.Upgrader
	nextReqID  atomic.Uint32
}

// NewServer creates a relay server.
func NewServer(baseDomain string) *Server {
	return &Server{
		registry:   NewRegistry(),
		baseDomain: baseDomain,
		upgrader: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  16 * 1024,
			WriteBufferSize: 16 * 1024,
		},
	}
}

// ServeHTTP is the main HTTP handler. It routes requests either to the
// tunnel WebSocket endpoint or to a tunnel-proxied request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Tunnel connect endpoint.
	if r.URL.Path == "/_tunnel/connect" {
		s.handleTunnelConnect(w, r)
		return
	}

	// Health check.
	if r.URL.Path == "/_health" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"tunnels": s.registry.Count(),
		})
		return
	}

	// Extract subdomain from Host header.
	subdomain := s.extractSubdomain(r.Host)
	if subdomain == "" {
		http.Error(w, "No tunnel found. Use a subdomain URL like https://abc123."+s.baseDomain, http.StatusNotFound)
		return
	}

	tc := s.registry.Lookup(subdomain)
	if tc == nil {
		http.Error(w, fmt.Sprintf("Tunnel '%s' not found or disconnected", subdomain), http.StatusBadGateway)
		return
	}

	s.proxyToTunnel(w, r, tc)
}

// handleTunnelConnect upgrades to WebSocket and registers the tunnel.
func (s *Server) handleTunnelConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[relay] websocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Wait for auth frame.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Printf("[relay] read auth: %v", err)
		return
	}
	conn.SetReadDeadline(time.Time{})

	frame, err := protocol.DecodeFrame(bytes.NewReader(msg))
	if err != nil || frame.Type != protocol.TypeAuth {
		s.sendError(conn, 0, "expected auth frame")
		return
	}

	var authMsg protocol.AuthMsg
	if err := protocol.DecodePayload(frame, &authMsg); err != nil {
		s.sendError(conn, 0, "invalid auth payload")
		return
	}

	// Generate subdomain.
	subdomain := tunnel.GenerateSubdomain()
	if authMsg.TunnelName != "" {
		// Use requested name if available (sanitize it).
		subdomain = sanitizeSubdomain(authMsg.TunnelName)
		// If already taken, append random suffix.
		if s.registry.Lookup(subdomain) != nil {
			subdomain = subdomain + "-" + tunnel.GenerateSubdomain()
		}
	}

	publicURL := fmt.Sprintf("https://%s.%s", subdomain, s.baseDomain)
	if s.baseDomain == "localhost" || strings.HasPrefix(s.baseDomain, "localhost:") {
		publicURL = fmt.Sprintf("http://%s.%s", subdomain, s.baseDomain)
	}

	// Create tunnel connection.
	var writeMu sync.Mutex
	tc := &TunnelConn{
		Subdomain: subdomain,
		AuthToken: authMsg.Token,
		SendFrame: func(data []byte) error {
			writeMu.Lock()
			defer writeMu.Unlock()
			return conn.WriteMessage(websocket.BinaryMessage, data)
		},
	}

	s.registry.Register(subdomain, tc)
	defer s.registry.Unregister(subdomain)

	log.Printf("[relay] tunnel registered: %s → %s", subdomain, publicURL)

	// Send TunnelReady.
	readyMsg := protocol.TunnelReadyMsg{
		Subdomain: subdomain,
		PublicURL: publicURL,
	}
	readyFrame, _ := protocol.EncodePayload(protocol.TypeTunnelReady, 0, readyMsg)
	var readyBuf bytes.Buffer
	readyFrame.Encode(&readyBuf)
	conn.WriteMessage(websocket.BinaryMessage, readyBuf.Bytes())

	// Read loop — receive response frames from the agent.
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[relay] tunnel %s disconnected: %v", subdomain, err)
			return
		}

		frame, err := protocol.DecodeFrame(bytes.NewReader(msg))
		if err != nil {
			log.Printf("[relay] decode error from %s: %v", subdomain, err)
			continue
		}

		switch frame.Type {
		case protocol.TypePing:
			pong := protocol.NewFrame(protocol.TypePong, 0, nil)
			var buf bytes.Buffer
			pong.Encode(&buf)
			tc.SendFrame(buf.Bytes())
		case protocol.TypePong:
			// Agent pong — connection alive.
		case protocol.TypeHTTPResponse:
			// Deliver response to the waiting HTTP handler.
			if ch, ok := tc.ReqChans.Load(frame.RequestID); ok {
				ch.(chan []byte) <- msg
			}
		}
	}
}

// proxyToTunnel serializes the HTTP request, sends it through the tunnel,
// and writes the response back to the original client.
func (s *Server) proxyToTunnel(w http.ResponseWriter, r *http.Request, tc *TunnelConn) {
	// Read the request body.
	body, err := io.ReadAll(io.LimitReader(r.Body, protocol.MaxPayloadSize))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	reqMsg := protocol.HTTPRequestMsg{
		Method:  r.Method,
		URL:     r.URL.String(),
		Path:    r.URL.Path,
		Query:   r.URL.RawQuery,
		Headers: r.Header,
		Body:    body,
		Host:    r.Host,
	}

	reqID := s.nextReqID.Add(1)
	frame, err := protocol.EncodePayload(protocol.TypeHTTPRequest, reqID, reqMsg)
	if err != nil {
		http.Error(w, "Failed to encode request", http.StatusInternalServerError)
		return
	}

	// Register response channel before sending.
	respCh := make(chan []byte, 1)
	tc.ReqChans.Store(reqID, respCh)
	defer tc.ReqChans.Delete(reqID)

	// Send to tunnel.
	var buf bytes.Buffer
	frame.Encode(&buf)
	if err := tc.SendFrame(buf.Bytes()); err != nil {
		http.Error(w, "Tunnel connection lost", http.StatusBadGateway)
		return
	}

	// Wait for response with timeout.
	select {
	case respMsg := <-respCh:
		respFrame, err := protocol.DecodeFrame(bytes.NewReader(respMsg))
		if err != nil {
			http.Error(w, "Invalid response from tunnel", http.StatusBadGateway)
			return
		}
		var resp protocol.HTTPResponseMsg
		if err := protocol.DecodePayload(respFrame, &resp); err != nil {
			http.Error(w, "Failed to decode response", http.StatusBadGateway)
			return
		}
		// Write response headers.
		for k, vals := range resp.Headers {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(resp.Body)

	case <-time.After(120 * time.Second):
		http.Error(w, "Tunnel response timeout", http.StatusGatewayTimeout)

	case <-r.Context().Done():
		// Client disconnected.
	}
}

// extractSubdomain extracts the subdomain from a Host header.
// e.g. "abc123.routa.dev" → "abc123", "abc123.localhost:8080" → "abc123"
func (s *Server) extractSubdomain(host string) string {
	// Strip port if present.
	h := host
	if idx := strings.LastIndex(h, ":"); idx != -1 {
		// Only strip if it looks like a port (after last colon).
		h = h[:idx]
	}
	baseDomain := s.baseDomain
	if idx := strings.LastIndex(baseDomain, ":"); idx != -1 {
		baseDomain = baseDomain[:idx]
	}

	if !strings.HasSuffix(h, "."+baseDomain) {
		return ""
	}
	sub := strings.TrimSuffix(h, "."+baseDomain)
	if sub == "" || strings.Contains(sub, ".") {
		return ""
	}
	return sub
}

// sendError sends a TypeError frame to the WebSocket connection.
func (s *Server) sendError(conn *websocket.Conn, reqID uint32, msg string) {
	errMsg := protocol.ErrorMsg{Code: 400, Message: msg}
	frame, _ := protocol.EncodePayload(protocol.TypeError, reqID, errMsg)
	var buf bytes.Buffer
	frame.Encode(&buf)
	conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}

// sanitizeSubdomain removes invalid characters from a requested tunnel name.
func sanitizeSubdomain(name string) string {
	var result strings.Builder
	for _, c := range strings.ToLower(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result.WriteRune(c)
		}
	}
	s := result.String()
	if s == "" {
		return tunnel.GenerateSubdomain()
	}
	return s
}
