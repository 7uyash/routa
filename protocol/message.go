package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// HTTPRequestMsg is the JSON payload inside a TypeHTTPRequest frame.
type HTTPRequestMsg struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Path    string              `json:"path"`
	Query   string              `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
	Host    string              `json:"host"`
}

// HTTPResponseMsg is the JSON payload inside a TypeHTTPResponse frame.
type HTTPResponseMsg struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
	Timing     TimingInfo          `json:"timing"`
}

// TimingInfo tracks the latency breakdown for a proxied request.
type TimingInfo struct {
	DNSLookup    time.Duration `json:"dns_lookup_ms"`
	TCPConnect   time.Duration `json:"tcp_connect_ms"`
	TLSHandshake time.Duration `json:"tls_handshake_ms"`
	FirstByte    time.Duration `json:"first_byte_ms"`
	Total        time.Duration `json:"total_ms"`
}

// AuthMsg is the JSON payload for TypeAuth frames.
type AuthMsg struct {
	Token      string `json:"token"`
	TunnelName string `json:"tunnel_name,omitempty"`
}

// TunnelReadyMsg is sent by the relay after a tunnel is established.
type TunnelReadyMsg struct {
	Subdomain string `json:"subdomain"`
	PublicURL string `json:"public_url"`
}

// ErrorMsg is sent for TypeError frames.
type ErrorMsg struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// EncodePayload marshals v to JSON and wraps it in a Frame.
func EncodePayload(typ uint8, requestID uint32, v any) (Frame, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return Frame{}, fmt.Errorf("marshal payload: %w", err)
	}
	return NewFrame(typ, requestID, data), nil
}

// DecodePayload unmarshals a Frame's payload into v.
func DecodePayload(f Frame, v any) error {
	if len(f.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(f.Payload, v); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	return nil
}
