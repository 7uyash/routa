package middleware

import (
	"encoding/json"
	"testing"

	"github.com/7uyash/routa/config"
)

func mutRule(name, matchPath, matchMethod string, req config.RequestMutation, resp config.ResponseMutation) config.MutationConfig {
	return config.MutationConfig{
		Name:     name,
		Match:    config.MatchConfig{Path: matchPath, Method: matchMethod},
		Request:  req,
		Response: resp,
	}
}

func TestMutatorSetHeader(t *testing.T) {
	m := NewMutator([]config.MutationConfig{
		mutRule("add-trace", "/api/*", "", config.RequestMutation{
			SetHeaders: map[string]string{"X-Trace-ID": "test-123"},
		}, config.ResponseMutation{}),
	})

	result := m.ApplyToRequest("GET", "/api/users", "", map[string][]string{}, nil)
	if got := result.Headers["X-Trace-ID"]; len(got) == 0 || got[0] != "test-123" {
		t.Errorf("expected X-Trace-ID: test-123, got %v", got)
	}
}

func TestMutatorRemoveHeader(t *testing.T) {
	m := NewMutator([]config.MutationConfig{
		mutRule("strip-auth", "/public/*", "", config.RequestMutation{
			RemoveHeaders: []string{"Authorization"},
		}, config.ResponseMutation{}),
	})

	result := m.ApplyToRequest("GET", "/public/page", "", map[string][]string{
		"Authorization": {"Bearer secret"},
		"Content-Type":  {"text/html"},
	}, nil)

	if _, ok := result.Headers["Authorization"]; ok {
		t.Error("Authorization header should have been removed")
	}
	if _, ok := result.Headers["Content-Type"]; !ok {
		t.Error("Content-Type header should still be present")
	}
}

func TestMutatorStripPathPrefix(t *testing.T) {
	m := NewMutator([]config.MutationConfig{
		mutRule("strip-v1", "/v1/*", "", config.RequestMutation{
			StripPathPrefix: "/v1",
		}, config.ResponseMutation{}),
	})

	result := m.ApplyToRequest("GET", "/v1/users/123", "", map[string][]string{}, nil)
	if result.Path != "/users/123" {
		t.Errorf("path: got %q, want /users/123", result.Path)
	}
}

func TestMutatorQueryMutation(t *testing.T) {
	m := NewMutator([]config.MutationConfig{
		mutRule("add-debug", "", "", config.RequestMutation{
			SetQuery:    map[string]string{"debug": "true"},
			RemoveQuery: []string{"private"},
		}, config.ResponseMutation{}),
	})

	result := m.ApplyToRequest("GET", "/search", "q=hello&private=secret", map[string][]string{}, nil)
	if result.Query == "" {
		t.Fatal("query should not be empty")
	}
	if !contains(result.Query, "debug=true") {
		t.Errorf("query %q should contain debug=true", result.Query)
	}
	if contains(result.Query, "private=secret") {
		t.Errorf("query %q should not contain private=secret", result.Query)
	}
}

func TestMutatorJSONBodyMutation(t *testing.T) {
	m := NewMutator([]config.MutationConfig{
		mutRule("elevate", "/admin/*", "POST", config.RequestMutation{
			SetBodyFields: map[string]string{"user.role": `"admin"`},
		}, config.ResponseMutation{}),
	})

	body, _ := json.Marshal(map[string]any{"user": map[string]any{"name": "alice", "role": "viewer"}})
	result := m.ApplyToRequest("POST", "/admin/users", "", map[string][]string{}, body)

	var out map[string]any
	if err := json.Unmarshal(result.Body, &out); err != nil {
		t.Fatalf("invalid JSON result: %v", err)
	}
	user := out["user"].(map[string]any)
	if user["role"] != "admin" {
		t.Errorf("role: got %v, want admin", user["role"])
	}
	if user["name"] != "alice" {
		t.Errorf("name should still be alice, got %v", user["name"])
	}
}

func TestMutatorMockResponse(t *testing.T) {
	m := NewMutator([]config.MutationConfig{
		mutRule("block-delete", "", "DELETE", config.RequestMutation{}, config.ResponseMutation{
			MockStatus:  403,
			MockBody:    `{"error":"deletes not allowed"}`,
			MockHeaders: map[string]string{"Content-Type": "application/json"},
		}),
	})

	result := m.ApplyToRequest("DELETE", "/api/users/1", "", map[string][]string{}, nil)
	if result.MockResponse == nil {
		t.Fatal("expected MockResponse, got nil")
	}
	if result.MockResponse.Status != 403 {
		t.Errorf("mock status: got %d, want 403", result.MockResponse.Status)
	}
}

func TestMutatorForceStatus(t *testing.T) {
	m := NewMutator([]config.MutationConfig{
		mutRule("override-status", "/legacy/*", "", config.RequestMutation{}, config.ResponseMutation{
			ForceStatus: 200,
		}),
	})

	outStatus, _, _ := m.ApplyToResponse("GET", "/legacy/old", 404, map[string][]string{}, nil)
	if outStatus != 200 {
		t.Errorf("force status: got %d, want 200", outStatus)
	}
}

func TestMutatorNoMatch(t *testing.T) {
	m := NewMutator([]config.MutationConfig{
		mutRule("api-only", "/api/*", "", config.RequestMutation{
			SetHeaders: map[string]string{"X-API": "true"},
		}, config.ResponseMutation{}),
	})

	result := m.ApplyToRequest("GET", "/health", "", map[string][]string{}, nil)
	if _, ok := result.Headers["X-API"]; ok {
		t.Error("X-API should not be set on non-matching path")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && (s[:len(sub)] == sub || s[len(s)-len(sub):] == sub || func() bool {
		for i := 1; i < len(s)-len(sub)+1; i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()))
}
