// Package webhook provides a webhook lab ("Connect Anything") for receiving, inspecting,
// testing, and replaying webhook deliveries from external services.
package webhook

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// KnownSignatureHeaders maps provider names to their webhook signature headers.
var KnownSignatureHeaders = map[string]string{
	"GitHub":   "X-Hub-Signature-256",
	"Stripe":   "Stripe-Signature",
	"Shopify":  "X-Shopify-Hmac-Sha256",
	"Discord":  "X-Signature-Ed25519",
	"Twilio":   "X-Twilio-Signature",
	"Slack":    "X-Slack-Signature",
	"SendGrid": "X-Twilio-Email-Event-Webhook-Signature",
	"PayPal":   "Paypal-Transmission-Sig",
	"Custom":   "X-Custom-Webhook-Signature",
}

// Endpoint represents a webhook endpoint created in the "Connect Anything" webhook lab.
type Endpoint struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"` // "Stripe", "GitHub", "Shopify", "Slack", "Custom", etc.
	Path      string    `json:"path"`     // e.g., "/webhook/abc123"
	Secret    string    `json:"secret,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Active    bool      `json:"active"` // ON/OFF Connection Toggle Control
}

// Delivery represents a single webhook delivery to an endpoint.
type Delivery struct {
	ID              string              `json:"id"`
	EndpointID      string              `json:"endpoint_id"`
	Timestamp       time.Time           `json:"timestamp"`
	Method          string              `json:"method"`
	Headers         map[string][]string `json:"headers"`
	Body            []byte              `json:"body"`
	ContentType     string              `json:"content_type"`
	SignatureHeader string              `json:"signature_header,omitempty"`
	SignatureValue  string              `json:"signature_value,omitempty"`
	Provider        string              `json:"provider,omitempty"`
	IsTestPayload   bool                `json:"is_test_payload"`
	Processed       bool                `json:"processed"`
}

// Lab manages webhook endpoints and their deliveries.
type Lab struct {
	mu         sync.RWMutex
	endpoints  map[string]*Endpoint
	deliveries map[string][]*Delivery // endpointID → deliveries
}

// NewLab creates a webhook Lab.
func NewLab() *Lab {
	return &Lab{
		endpoints:  make(map[string]*Endpoint),
		deliveries: make(map[string][]*Delivery),
	}
}

// CreateEndpoint creates a new webhook endpoint for any provider.
func (l *Lab) CreateEndpoint(name, provider, secret string) *Endpoint {
	l.mu.Lock()
	defer l.mu.Unlock()

	if provider == "" {
		provider = "Custom"
	}

	id := generateWebhookID()
	ep := &Endpoint{
		ID:        id,
		Name:      name,
		Provider:  provider,
		Path:      "/webhook/" + id,
		Secret:    secret,
		CreatedAt: time.Now(),
		Active:    true,
	}
	l.endpoints[id] = ep
	l.deliveries[id] = make([]*Delivery, 0)
	return ep
}

// ToggleEndpoint toggles the active ON/OFF connection state for an endpoint.
func (l *Lab) ToggleEndpoint(id string) (bool, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ep, ok := l.endpoints[id]
	if !ok {
		return false, false
	}

	ep.Active = !ep.Active
	return ep.Active, true
}

// SimulateTestDelivery fires a simulated test connection payload to verify endpoint handler logic.
func (l *Lab) SimulateTestDelivery(endpointID string) (*Delivery, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ep, ok := l.endpoints[endpointID]
	if !ok {
		return nil, false
	}

	sigHeader := KnownSignatureHeaders[ep.Provider]
	if sigHeader == "" {
		sigHeader = "X-Webhook-Signature"
	}

	testPayload := fmt.Sprintf(`{"event": "test.connection", "provider": "%s", "timestamp": %d, "status": "simulated_success"}`, ep.Provider, time.Now().Unix())

	d := &Delivery{
		ID:              generateWebhookID(),
		EndpointID:      endpointID,
		Timestamp:       time.Now(),
		Method:          "POST",
		Headers:         map[string][]string{"Content-Type": {"application/json"}, sigHeader: {"test_simulated_sig_123"}},
		Body:            []byte(testPayload),
		ContentType:     "application/json",
		SignatureHeader: sigHeader,
		SignatureValue:  "test_simulated_sig_123",
		Provider:        ep.Provider,
		IsTestPayload:   true,
		Processed:       true,
	}

	l.deliveries[endpointID] = append(l.deliveries[endpointID], d)
	return d, true
}

// GetEndpoint returns an endpoint by ID.
func (l *Lab) GetEndpoint(id string) *Endpoint {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.endpoints[id]
}

// ListEndpoints returns all webhook endpoints.
func (l *Lab) ListEndpoints() []*Endpoint {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]*Endpoint, 0, len(l.endpoints))
	for _, ep := range l.endpoints {
		result = append(result, ep)
	}
	return result
}

// RecordDelivery records a webhook delivery to an endpoint.
func (l *Lab) RecordDelivery(endpointID string, method string, headers map[string][]string, body []byte) *Delivery {
	l.mu.Lock()
	defer l.mu.Unlock()

	ep := l.endpoints[endpointID]
	provider := "Custom"
	if ep != nil && ep.Provider != "" {
		provider = ep.Provider
	} else {
		detected, _, _ := detectProvider(headers)
		if detected != "" {
			provider = detected
		}
	}

	_, sigHeader, sigValue := detectProvider(headers)

	contentType := ""
	if ct, ok := headers["Content-Type"]; ok && len(ct) > 0 {
		contentType = ct[0]
	}

	d := &Delivery{
		ID:              generateWebhookID(),
		EndpointID:      endpointID,
		Timestamp:       time.Now(),
		Method:          method,
		Headers:         headers,
		Body:            body,
		ContentType:     contentType,
		SignatureHeader: sigHeader,
		SignatureValue:  sigValue,
		Provider:        provider,
	}

	l.deliveries[endpointID] = append(l.deliveries[endpointID], d)
	return d
}

// GetDeliveries returns all deliveries for an endpoint (newest first).
func (l *Lab) GetDeliveries(endpointID string) []*Delivery {
	l.mu.RLock()
	defer l.mu.RUnlock()
	deliveries := l.deliveries[endpointID]
	result := make([]*Delivery, len(deliveries))
	for i, d := range deliveries {
		result[len(deliveries)-1-i] = d
	}
	return result
}

// MatchPath checks if a request path matches any active webhook endpoint.
func (l *Lab) MatchPath(path string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for id, ep := range l.endpoints {
		if ep.Active && strings.HasPrefix(path, ep.Path) {
			return id
		}
	}
	return ""
}

// DeleteEndpoint removes a webhook endpoint and its deliveries.
func (l *Lab) DeleteEndpoint(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.endpoints, id)
	delete(l.deliveries, id)
}

func detectProvider(headers map[string][]string) (provider, sigHeader, sigValue string) {
	for prov, header := range KnownSignatureHeaders {
		for k, vals := range headers {
			if strings.EqualFold(k, header) && len(vals) > 0 {
				return prov, header, vals[0]
			}
		}
	}
	return "", "", ""
}

func generateWebhookID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}
