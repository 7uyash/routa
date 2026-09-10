// Package mock provides the Mock Lab for creating, editing, and serving local mock API endpoints
// and 1-click conversion from captured live traffic into mocks.
package mock

import (
	crand "crypto/rand"
	"encoding/hex"
	mrand "math/rand/v2"
	"sync"
	"time"
)

	"github.com/7uyash/routa/discovery"
	"github.com/7uyash/routa/recorder"
)

// MockRule represents a mock API endpoint served by Routa.
type MockRule struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Method    string            `json:"method"` // "GET", "POST", "*", etc.
	Path      string            `json:"path"`   // Exact path or wildcard pattern
	Status    int               `json:"status"` // HTTP status code (e.g. 200, 500, 503)
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	DelayMs   int               `json:"delay_ms"`   // Artificial response latency
	ErrorRate float64           `json:"error_rate"` // Probability of returning 500 error (0.0 - 1.0)
	Active    bool              `json:"active"`
	CreatedAt time.Time         `json:"created_at"`
}

// Lab manages mock API endpoints.
type Lab struct {
	mu    sync.RWMutex
	rules map[string]*MockRule
}

// NewLab creates a new Mock Lab instance.
func NewLab() *Lab {
	return &Lab{
		rules: make(map[string]*MockRule),
	}
}

// CreateRule adds a new mock rule.
func (l *Lab) CreateRule(name, method, path string, status int, headers map[string]string, body string, delayMs int, active bool) *MockRule {
	l.mu.Lock()
	defer l.mu.Unlock()

	id := generateMockID()
	if headers == nil {
		headers = map[string]string{"Content-Type": "application/json"}
	}

	rule := &MockRule{
		ID:        id,
		Name:      name,
		Method:    method,
		Path:      path,
		Status:    status,
		Headers:   headers,
		Body:      body,
		DelayMs:   delayMs,
		Active:    active,
		CreatedAt: time.Now(),
	}

	l.rules[id] = rule
	return rule
}

// CreateFromRequest converts a recorded request entry into a local mock endpoint (1-Click Traffic-to-Mock).
func (l *Lab) CreateFromRequest(entry *recorder.Entry) *MockRule {
	l.mu.Lock()
	defer l.mu.Unlock()

	id := generateMockID()
	normPath := discovery.NormalizePath(entry.Path)

	hdrs := make(map[string]string)
	if ct, ok := entry.ResponseHeaders["Content-Type"]; ok && len(ct) > 0 {
		hdrs["Content-Type"] = ct[0]
	} else {
		hdrs["Content-Type"] = "application/json"
	}

	status := entry.StatusCode
	if status == 0 {
		status = 200
	}

	rule := &MockRule{
		ID:        id,
		Name:      "Mock for " + entry.Method + " " + normPath,
		Method:    entry.Method,
		Path:      entry.Path,
		Status:    status,
		Headers:   hdrs,
		Body:      string(entry.ResponseBody),
		DelayMs:   0,
		Active:    true,
		CreatedAt: time.Now(),
	}

	l.rules[id] = rule
	return rule
}

// UpdateRule updates an existing mock rule.
func (l *Lab) UpdateRule(id, name, method, path string, status int, headers map[string]string, body string, delayMs int, active bool) (*MockRule, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	rule, ok := l.rules[id]
	if !ok {
		return nil, false
	}

	rule.Name = name
	rule.Method = method
	rule.Path = path
	rule.Status = status
	if headers != nil {
		rule.Headers = headers
	}
	rule.Body = body
	rule.DelayMs = delayMs
	rule.Active = active

	return rule, true
}

// DeleteRule removes a mock rule by ID.
func (l *Lab) DeleteRule(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.rules[id]; ok {
		delete(l.rules, id)
		return true
	}
	return false
}

// ListRules returns all mock rules.
func (l *Lab) ListRules() []*MockRule {
	l.mu.RLock()
	defer l.mu.RUnlock()

	res := make([]*MockRule, 0, len(l.rules))
	for _, rule := range l.rules {
		res = append(res, rule)
	}
	return res
}

// MatchRequest finds an active mock rule matching the incoming HTTP method and path.
func (l *Lab) MatchRequest(method, path string) *MockRule {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, rule := range l.rules {
		if !rule.Active {
			continue
		}
		if (rule.Method == "*" || rule.Method == method) && matchPathPattern(rule.Path, path) {
			return rule
		}
	}
	return nil
}

// ServeMock executes a mock rule response, applying optional delay and error rate.
func (r *MockRule) ServeMock() (int, map[string]string, []byte) {
	if r.DelayMs > 0 {
		time.Sleep(time.Duration(r.DelayMs) * time.Millisecond)
	}

	if r.ErrorRate > 0 && mrand.Float64() < r.ErrorRate {
		return 500, map[string]string{"Content-Type": "application/json"}, []byte(`{"error": "Simulated 500 error from Mock Lab"}`)
	}

	return r.Status, r.Headers, []byte(r.Body)
}

func matchPathPattern(pattern, path string) bool {
	if pattern == path || pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}
	return false
}

func generateMockID() string {
	b := make([]byte, 6)
	crand.Read(b)
	return "mock_" + hex.EncodeToString(b)
}
