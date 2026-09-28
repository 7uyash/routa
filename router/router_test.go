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

// TestRouterUsesCanonicalMatching pins the router to traffic.MatchPath. The
// router used to have its own matcher, which treated "*" and "/api*" as literal
// paths, so a route with either of those patterns silently matched nothing.
func TestRouterUsesCanonicalMatching(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"bare star means any path", "*", "/anything", true},
		{"bare star means any path at the root", "*", "/", true},
		{"string prefix wildcard", "/api*", "/api/users", true},
		{"string prefix wildcard ignores the boundary", "/api*", "/apifoo", true},
		{"segment wildcard matches the prefix itself", "/api/*", "/api", true},
		{"segment wildcard respects the boundary", "/api/*", "/apifoo", false},
		{"exact match", "/health", "/health", true},
		{"exact match does not spill over", "/health", "/healthy", false},
		{"empty pattern matches all", "", "/anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(Route{Pattern: tt.pattern, Target: "http://hit"})
			got := r.Match(tt.path) != ""
			if got != tt.want {
				t.Errorf("Match(%q) with pattern %q = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}
