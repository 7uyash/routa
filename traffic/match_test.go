package traffic

import "testing"

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// Exact matching.
		{"exact match", "/health", "/health", true},
		{"exact does not match a subpath", "/health", "/health/live", false},
		{"exact does not match a sibling", "/health", "/healthy", false},
		{"exact is case sensitive", "/Health", "/health", false},
		{"root path", "/", "/", true},

		// Match-all patterns.
		{"empty pattern matches all", "", "/anything", true},
		{"bare star matches all", "*", "/anything", true},
		{"catch-all matches all", "/*", "/anything", true},
		{"catch-all matches root", "/*", "/", true},
		{"bare star matches root", "*", "/", true},

		// Segment-aware "/prefix/*".
		{"segment wildcard matches the prefix", "/api/*", "/api", true},
		{"segment wildcard matches below the prefix", "/api/*", "/api/users", true},
		{"segment wildcard matches deep paths", "/api/*", "/api/v1/users/1", true},
		{"segment wildcard respects the boundary", "/api/*", "/apifoo", false},
		{"segment wildcard does not match a sibling", "/api/*", "/other", false},
		{"segment wildcard needs a real prefix", "/*", "/anything", true},
		{"single segment wildcard", "/api/*", "/api/", true},

		// Plain string prefix "/prefix*".
		{"string prefix matches the prefix itself", "/api*", "/api", true},
		{"string prefix matches below", "/api*", "/api/users", true},
		{"string prefix ignores the boundary", "/api*", "/apifoo", true},
		{"string prefix rejects an unrelated path", "/api*", "/other", false},

		// A star that is not a suffix is literal.
		{"interior star is literal", "/a*/b", "/a*/b", true},
		{"interior star does not glob", "/a*/b", "/ax/b", false},
		{"lone slash is not a wildcard", "/", "/anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchPath(tt.pattern, tt.path); got != tt.want {
				t.Errorf("MatchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestMatchMethod(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		method  string
		want    bool
	}{
		{"exact match", "GET", "GET", true},
		{"case insensitive", "get", "GET", true},
		{"case insensitive the other way", "GET", "get", true},
		{"different method", "GET", "POST", false},
		{"empty pattern matches any method", "", "DELETE", true},
		{"star matches any method", "*", "PATCH", true},
		{"empty request method matches a specific pattern", "GET", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchMethod(tt.pattern, tt.method); got != tt.want {
				t.Errorf("MatchMethod(%q, %q) = %v, want %v", tt.pattern, tt.method, got, tt.want)
			}
		})
	}
}

func TestMatchRequest(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		pattern string
		req     Request
		want    bool
	}{
		{"method and path both match", "GET", "/api/*", Request{Method: "GET", Path: "/api/users"}, true},
		{"method mismatch", "POST", "/api/*", Request{Method: "GET", Path: "/api/users"}, false},
		{"path mismatch", "GET", "/api/*", Request{Method: "GET", Path: "/other"}, false},
		{"empty method and path match anything", "", "", Request{Method: "GET", Path: "/anything"}, true},
		{"wildcards match anything", "*", "*", Request{Method: "DELETE", Path: "/x/y"}, true},
		{"lowercase method pattern", "get", "/api/*", Request{Method: "GET", Path: "/api/users"}, true},
		{"segment boundary is respected", "GET", "/api/*", Request{Method: "GET", Path: "/apifoo"}, false},
		{"prefix matches itself", "GET", "/api/*", Request{Method: "GET", Path: "/api"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchRequest(tt.method, tt.pattern, tt.req); got != tt.want {
				t.Errorf("MatchRequest(%q, %q, %+v) = %v, want %v", tt.method, tt.pattern, tt.req, got, tt.want)
			}
		})
	}
}

// TestMatchPathIsStableAcrossFeatures documents the point of centralising the
// matcher: the four features that used to disagree now agree. Each case is a
// (pattern, path) pair where at least one feature used to answer differently.
func TestMatchPathIsStableAcrossFeatures(t *testing.T) {
	previouslyDivergent := []struct {
		pattern string
		path    string
		want    bool
		note    string
	}{
		{"/api/*", "/api", true, "router matched the prefix, mutation/simulation/mock did not"},
		{"*", "/anything", true, "the dashboard renders * as 'any path', the router matched it literally"},
		{"/api*", "/api/users", true, "mutation/simulation/mock globbed, the router matched it literally"},
		{"", "/anything", true, "config.MatchConfig documents empty as 'match all', mock never matched"},
	}

	for _, c := range previouslyDivergent {
		if got := MatchPath(c.pattern, c.path); got != c.want {
			t.Errorf("MatchPath(%q, %q) = %v, want %v (%s)", c.pattern, c.path, got, c.want, c.note)
		}
	}
}
