package middleware

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/traffic"
)

// Mutator applies traffic mutation rules to requests and responses.
type Mutator struct {
	mu    sync.RWMutex
	rules []config.MutationConfig
}

// NewMutator creates a Mutator from a slice of rules.
func NewMutator(rules []config.MutationConfig) *Mutator {
	cp := make([]config.MutationConfig, len(rules))
	copy(cp, rules)
	return &Mutator{rules: cp}
}

// SetRules replaces the rule set at runtime (hot-reload from dashboard).
func (m *Mutator) SetRules(rules []config.MutationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]config.MutationConfig, len(rules))
	copy(cp, rules)
	m.rules = cp
}

// Rules returns a copy of the current rule set.
func (m *Mutator) Rules() []config.MutationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make([]config.MutationConfig, len(m.rules))
	copy(cp, m.rules)
	return cp
}

// MutatedRequest is the result of applying request mutations.
type MutatedRequest struct {
	Request traffic.Request
	// If non-nil, skip forwarding and return this mock response immediately.
	MockResponse *traffic.Response
}

// ApplyToRequest applies all matching rules to a request and returns the mutated version.
func (m *Mutator) ApplyToRequest(req traffic.Request) MutatedRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := MutatedRequest{
		Request: req,
	}
	result.Request.Headers = copyHeaders(req.Headers)

	for _, rule := range m.rules {
		if !matchesRule(rule.Match, result.Request.Method, result.Request.Path) {
			continue
		}

		// Check for mock response first — if set, short-circuit.
		if rule.Response.MockStatus != 0 {
			mockHdrs := make(http.Header)
			if rule.Response.MockHeaders != nil {
				for k, v := range rule.Response.MockHeaders {
					mockHdrs.Set(k, v)
				}
			} else {
				mockHdrs.Set("Content-Type", "application/json")
			}
			result.MockResponse = &traffic.Response{
				StatusCode: rule.Response.MockStatus,
				Headers:    mockHdrs,
				Body:       []byte(rule.Response.MockBody),
			}
			return result
		}

		req := rule.Request

		// Set / override headers.
		for k, v := range req.SetHeaders {
			result.Request.Headers[k] = []string{v}
			if canonical := http.CanonicalHeaderKey(k); canonical != k {
				result.Request.Headers[canonical] = []string{v}
			}
		}
		// Remove headers.
		for _, k := range req.RemoveHeaders {
			delete(result.Request.Headers, k)
			delete(result.Request.Headers, http.CanonicalHeaderKey(k))
			delete(result.Request.Headers, strings.ToLower(k))
		}

		// Path rewrite.
		if req.StripPathPrefix != "" {
			result.Request.Path = strings.TrimPrefix(result.Request.Path, req.StripPathPrefix)
			if result.Request.Path == "" {
				result.Request.Path = "/"
			}
		}
		if req.ReplacePath != "" {
			result.Request.Path = req.ReplacePath
		}

		// Query mutation.
		if len(req.SetQuery) > 0 || len(req.RemoveQuery) > 0 {
			result.Request.Query = mutateQuery(result.Request.Query, req.SetQuery, req.RemoveQuery)
		}

		// JSON body field mutations.
		if len(req.SetBodyFields) > 0 {
			result.Request.Body = mutateJSONBody(result.Request.Body, req.SetBodyFields)
		}
	}

	return result
}

// ApplyToResponse applies all matching rules to a response (headers, status).
// Returns mutated headers and status code.
func (m *Mutator) ApplyToResponse(req traffic.Request, resp *traffic.Response) {
	if resp == nil {
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if resp.Headers == nil {
		resp.Headers = make(http.Header)
	}

	for _, rule := range m.rules {
		if !matchesRule(rule.Match, req.Method, req.Path) {
			continue
		}
		ruleResp := rule.Response

		for k, v := range ruleResp.SetHeaders {
			resp.Headers[k] = []string{v}
			if canonical := http.CanonicalHeaderKey(k); canonical != k {
				resp.Headers[canonical] = []string{v}
			}
		}
		for _, k := range ruleResp.RemoveHeaders {
			delete(resp.Headers, k)
			delete(resp.Headers, http.CanonicalHeaderKey(k))
			delete(resp.Headers, strings.ToLower(k))
		}
		if ruleResp.ForceStatus != 0 {
			resp.StatusCode = ruleResp.ForceStatus
		}
	}
}

// --- helpers ---

func matchesRule(match config.MatchConfig, method, path string) bool {
	if match.Method != "" && !strings.EqualFold(match.Method, method) {
		return false
	}
	if match.Path != "" {
		pattern := match.Path
		if strings.HasSuffix(pattern, "*") {
			if !strings.HasPrefix(path, strings.TrimSuffix(pattern, "*")) {
				return false
			}
		} else {
			if path != pattern {
				return false
			}
		}
	}
	return true
}

func copyHeaders(h map[string][]string) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, v := range h {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

func mutateQuery(raw string, set map[string]string, remove []string) string {
	vals, _ := url.ParseQuery(raw)
	if vals == nil {
		vals = url.Values{}
	}
	for k, v := range set {
		vals.Set(k, v)
	}
	for _, k := range remove {
		vals.Del(k)
	}
	return vals.Encode()
}

// mutateJSONBody applies dot-notation field mutations to a JSON body.
// e.g. "user.role" → "admin"
func mutateJSONBody(body []byte, fields map[string]string) []byte {
	if len(body) == 0 {
		return body
	}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return body // not JSON, return unchanged
	}
	for dotPath, rawVal := range fields {
		var newVal any
		if err := json.Unmarshal([]byte(rawVal), &newVal); err != nil {
			newVal = rawVal // treat as plain string
		}
		root = setDotPath(root, strings.Split(dotPath, "."), newVal)
	}
	out, err := json.Marshal(root)
	if err != nil {
		return body
	}
	return out
}

// setDotPath sets a nested field in an any (map/slice structure).
func setDotPath(node any, parts []string, val any) any {
	if len(parts) == 0 {
		return val
	}
	m, ok := node.(map[string]any)
	if !ok {
		m = make(map[string]any)
	}
	key := parts[0]
	if len(parts) == 1 {
		m[key] = val
	} else {
		m[key] = setDotPath(m[key], parts[1:], val)
	}
	return m
}
