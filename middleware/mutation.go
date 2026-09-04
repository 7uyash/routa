package middleware

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/7uyash/routa/config"
)

// Mutator applies traffic mutation rules to requests and responses.
type Mutator struct {
	rules []config.MutationConfig
}

// NewMutator creates a Mutator from a slice of rules.
func NewMutator(rules []config.MutationConfig) *Mutator {
	return &Mutator{rules: rules}
}

// SetRules replaces the rule set at runtime (hot-reload from dashboard).
func (m *Mutator) SetRules(rules []config.MutationConfig) {
	m.rules = rules
}

// Rules returns a copy of the current rule set.
func (m *Mutator) Rules() []config.MutationConfig {
	cp := make([]config.MutationConfig, len(m.rules))
	copy(cp, m.rules)
	return cp
}

// MutatedRequest is the result of applying request mutations.
type MutatedRequest struct {
	Method  string
	Path    string
	Query   string
	Headers map[string][]string
	Body    []byte
	// If non-nil, skip forwarding and return this mock response immediately.
	MockResponse *MockResponse
}

// MockResponse represents a canned response returned without touching the local service.
type MockResponse struct {
	Status  int
	Headers map[string]string
	Body    []byte
}

// ApplyToRequest applies all matching rules to a request and returns the mutated version.
func (m *Mutator) ApplyToRequest(method, path, query string, headers map[string][]string, body []byte) MutatedRequest {
	result := MutatedRequest{
		Method:  method,
		Path:    path,
		Query:   query,
		Headers: copyHeaders(headers),
		Body:    body,
	}

	for _, rule := range m.rules {
		if !matchesRule(rule.Match, method, path) {
			continue
		}

		// Check for mock response first — if set, short-circuit.
		if rule.Response.MockStatus != 0 {
			mockHdrs := rule.Response.MockHeaders
			if mockHdrs == nil {
				mockHdrs = map[string]string{"Content-Type": "application/json"}
			}
			result.MockResponse = &MockResponse{
				Status:  rule.Response.MockStatus,
				Headers: mockHdrs,
				Body:    []byte(rule.Response.MockBody),
			}
			return result
		}

		req := rule.Request

		// Set / override headers.
		for k, v := range req.SetHeaders {
			result.Headers[k] = []string{v}
		}
		// Remove headers.
		for _, k := range req.RemoveHeaders {
			delete(result.Headers, k)
			// Also try canonical form.
			delete(result.Headers, http2canonical(k))
		}

		// Path rewrite.
		if req.StripPathPrefix != "" {
			result.Path = strings.TrimPrefix(result.Path, req.StripPathPrefix)
			if result.Path == "" {
				result.Path = "/"
			}
		}
		if req.ReplacePath != "" {
			result.Path = req.ReplacePath
		}

		// Query mutation.
		if len(req.SetQuery) > 0 || len(req.RemoveQuery) > 0 {
			result.Query = mutateQuery(result.Query, req.SetQuery, req.RemoveQuery)
		}

		// JSON body field mutations.
		if len(req.SetBodyFields) > 0 {
			result.Body = mutateJSONBody(result.Body, req.SetBodyFields)
		}
	}

	return result
}

// ApplyToResponse applies all matching rules to a response (headers, status).
// Returns mutated headers and status code.
func (m *Mutator) ApplyToResponse(method, path string, status int, headers map[string][]string, body []byte) (int, map[string][]string, []byte) {
	outStatus := status
	outHeaders := copyHeaders(headers)
	outBody := body

	for _, rule := range m.rules {
		if !matchesRule(rule.Match, method, path) {
			continue
		}
		resp := rule.Response

		for k, v := range resp.SetHeaders {
			outHeaders[k] = []string{v}
		}
		for _, k := range resp.RemoveHeaders {
			delete(outHeaders, k)
			delete(outHeaders, http2canonical(k))
		}
		if resp.ForceStatus != 0 {
			outStatus = resp.ForceStatus
		}
	}

	return outStatus, outHeaders, outBody
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

// http2canonical converts a lowercase header name to Go's canonical form.
func http2canonical(s string) string {
	return fmt.Sprintf("%s%s", strings.ToUpper(s[:1]), s[1:])
}
