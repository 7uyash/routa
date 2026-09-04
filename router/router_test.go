package router

import "testing"

func TestRouterMatchCatchAll(t *testing.T) {
	r := NewSingle("http://localhost:3000")
	if got := r.Match("/anything/here"); got != "http://localhost:3000" {
		t.Errorf("Match(/anything/here) = %q, want http://localhost:3000", got)
	}
}

func TestRouterMatchExact(t *testing.T) {
	r := New(Route{Pattern: "/health", Target: "http://localhost:3000"})
	if got := r.Match("/health"); got != "http://localhost:3000" {
		t.Errorf("Match(/health) = %q", got)
	}
	if got := r.Match("/other"); got != "" {
		t.Errorf("Match(/other) = %q, want empty", got)
	}
}

func TestRouterMatchPrefix(t *testing.T) {
	r := New(
		Route{Pattern: "/api/*", Target: "http://localhost:3000"},
		Route{Pattern: "/admin/*", Target: "http://localhost:4000"},
		Route{Pattern: "/*", Target: "http://localhost:5000"},
	)

	tests := []struct {
		path   string
		target string
	}{
		{"/api/users", "http://localhost:3000"},
		{"/api/posts/1", "http://localhost:3000"},
		{"/admin/dashboard", "http://localhost:4000"},
		{"/other", "http://localhost:5000"},
	}

	for _, tt := range tests {
		got := r.Match(tt.path)
		if got != tt.target {
			t.Errorf("Match(%q) = %q, want %q", tt.path, got, tt.target)
		}
	}
}

func TestRouterSetRoutes(t *testing.T) {
	r := NewSingle("http://localhost:3000")
	r.SetRoutes([]Route{
		{Pattern: "/api/*", Target: "http://localhost:8080"},
	})
	if got := r.Match("/api/v1"); got != "http://localhost:8080" {
		t.Errorf("after SetRoutes, Match(/api/v1) = %q", got)
	}
	if got := r.Match("/other"); got != "" {
		t.Errorf("after SetRoutes, Match(/other) = %q, want empty", got)
	}
}

func TestRouterRoutes(t *testing.T) {
	routes := []Route{
		{Pattern: "/a/*", Target: "http://localhost:1000"},
		{Pattern: "/b/*", Target: "http://localhost:2000"},
	}
	r := New(routes...)
	got := r.Routes()
	if len(got) != 2 {
		t.Errorf("Routes() len = %d, want 2", len(got))
	}
}
