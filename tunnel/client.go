// Package tunnel manages the WebSocket connection between the local agent
// and the public relay server, with auto-reconnect and heartbeat.
package tunnel

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"routa/protocol"

	"github.com/gorilla/websocket"
)

// State represents the tunnel connection state.
type State int

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
)

func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	default:
		return "unknown"
	}
}

// Client manages a persistent WebSocket tunnel to the relay server.
type Client struct {
	relayURL   string
	authToken  string
	tunnelName string

	conn     *websocket.Conn
	connMu   sync.Mutex
	state    atomic.Int32
	stopCh   chan struct{}
	doneCh   chan struct{}

	// Stats
	requestCount   atomic.Int64
	reconnectCount atomic.Int64
	connectedAt    time.Time

	// Assigned by relay after auth
	Subdomain string
	PublicURL string

	// Callbacks
	OnFrame      func(protocol.Frame) // called for each incoming frame
	OnConnect    func()
	OnDisconnect func()

	// Pending response channels: requestID → chan Frame
	pending   sync.Map
	nextReqID atomic.Uint32
}

// NewClient creates a tunnel client. Call Connect() to start.
func NewClient(relayURL, authToken, tunnelName string) *Client {
	return &Client{
		relayURL:   relayURL,
		authToken:  authToken,
		tunnelName: tunnelName,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// State returns the current tunnel state.
func (c *Client) State() State {
	return State(c.state.Load())
}

// Stats returns tunnel statistics.
func (c *Client) Stats() Stats {
	return Stats{
		State:          c.State(),
		RequestCount:   c.requestCount.Load(),
		ReconnectCount: c.reconnectCount.Load(),
		ConnectedAt:    c.connectedAt,
		Subdomain:      c.Subdomain,
		PublicURL:      c.PublicURL,
	}
}

// Stats holds tunnel statistics.
type Stats struct {
	State          State     `json:"state"`
	RequestCount   int64     `json:"request_count"`
	ReconnectCount int64     `json:"reconnect_count"`
	ConnectedAt    time.Time `json:"connected_at"`
	Subdomain      string    `json:"subdomain"`
	PublicURL      string    `json:"public_url"`
}

// Connect establishes the tunnel and runs the read loop. Blocks until
// the tunnel is stopped or a fatal error occurs.
func (c *Client) Connect(ctx context.Context) error {
	defer close(c.doneCh)

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopCh:
			return nil
		default:
		}

		c.state.Store(int32(StateConnecting))
		err := c.dial(ctx)
		if err != nil {
			log.Printf("[tunnel] connection failed: %v", err)
		}

		c.state.Store(int32(StateDisconnected))
		if c.OnDisconnect != nil {
			c.OnDisconnect()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopCh:
			return nil
		case <-time.After(backoff):
			c.reconnectCount.Add(1)
			backoff = min(backoff*2, maxBackoff)
		}
	}
}

// Stop gracefully shuts down the tunnel.
func (c *Client) Stop() {
	close(c.stopCh)
	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connMu.Unlock()
	<-c.doneCh
}

// SendFrame sends a frame through the tunnel.
func (c *Client) SendFrame(f protocol.Frame) error {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	var buf bytes.Buffer
	if err := f.Encode(&buf); err != nil {
		return err
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
}

// NextRequestID returns a unique request ID for multiplexing.
func (c *Client) NextRequestID() uint32 {
	return c.nextReqID.Add(1)
}

// dial connects to the relay, authenticates, and runs the read loop.
func (c *Client) dial(ctx context.Context) error {
	header := http.Header{}
	header.Set("X-Routa-Token", c.authToken)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.relayURL+"/_tunnel/connect", header)
	if err != nil {
		return fmt.Errorf("dial relay: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	// Send auth frame.
	authMsg := protocol.AuthMsg{
		Token:      c.authToken,
		TunnelName: c.tunnelName,
	}
	authFrame, err := protocol.EncodePayload(protocol.TypeAuth, 0, authMsg)
	if err != nil {
		conn.Close()
		return fmt.Errorf("encode auth: %w", err)
	}

	var authBuf bytes.Buffer
	if err := authFrame.Encode(&authBuf); err != nil {
		conn.Close()
		return err
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, authBuf.Bytes()); err != nil {
		conn.Close()
		return fmt.Errorf("send auth: %w", err)
	}

	// Wait for AuthOK or AuthFail.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return fmt.Errorf("read auth response: %w", err)
	}
	conn.SetReadDeadline(time.Time{})

	frame, err := protocol.DecodeFrame(bytes.NewReader(msg))
	if err != nil {
		conn.Close()
		return fmt.Errorf("decode auth response: %w", err)
	}

	if frame.Type == protocol.TypeAuthFail {
		var errMsg protocol.ErrorMsg
		protocol.DecodePayload(frame, &errMsg)
		conn.Close()
		return fmt.Errorf("auth failed: %s", errMsg.Message)
	}

	if frame.Type == protocol.TypeTunnelReady {
		var ready protocol.TunnelReadyMsg
		if err := protocol.DecodePayload(frame, &ready); err != nil {
			conn.Close()
			return fmt.Errorf("decode tunnel ready: %w", err)
		}
		c.Subdomain = ready.Subdomain
		c.PublicURL = ready.PublicURL
	}

	c.state.Store(int32(StateConnected))
	c.connectedAt = time.Now()

	if c.OnConnect != nil {
		c.OnConnect()
	}

	// Start heartbeat.
	go c.heartbeat(conn)

	// Read loop.
	return c.readLoop(conn)
}

// readLoop reads frames from the WebSocket and dispatches them.
func (c *Client) readLoop(conn *websocket.Conn) error {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		frame, err := protocol.DecodeFrame(bytes.NewReader(msg))
		if err != nil {
			log.Printf("[tunnel] decode error: %v", err)
			continue
		}

		switch frame.Type {
		case protocol.TypePing:
			// Respond with pong.
			pong := protocol.NewFrame(protocol.TypePong, 0, nil)
			c.SendFrame(pong)
		case protocol.TypePong:
			// Heartbeat response — connection is alive.
		default:
			c.requestCount.Add(1)
			if c.OnFrame != nil {
				c.OnFrame(frame)
			}
		}
	}
}

// heartbeat sends periodic pings to detect dead connections.
func (c *Client) heartbeat(conn *websocket.Conn) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			ping := protocol.NewFrame(protocol.TypePing, 0, nil)
			if err := c.SendFrame(ping); err != nil {
				log.Printf("[tunnel] heartbeat failed: %v", err)
				conn.Close()
				return
			}
		}
	}
}

// GenerateSubdomain creates a random 6-character subdomain.
func GenerateSubdomain() string {
	b := make([]byte, 3)
	rand.Read(b)
	return hex.EncodeToString(b)
}
