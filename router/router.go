// Package router matches incoming request paths to local service targets.
// Phase 1 supports a single target; the structure is ready for multi-service
// routing in Phase 2.
package router

import (
	"strings"
	"sync"
)

// Route maps a path pattern to a local service target.
type Route struct {
	Pattern string `json:"pattern" yaml:"pattern"` // e.g. "/api/*", "/*"
	Target  string `json:"target" yaml:"target"`   // e.g. "http://localhost:3000"
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
}

// Router manages route matching from path patterns to local targets.
type Router struct {
	mu     sync.RWMutex
	routes []Route
}

// New creates a Router with the given routes. If no routes are provided,
// a catch-all route must be added with AddRoute before matching.
func New(routes ...Route) *Router {
	return &Router{
		routes: routes,
	}
}

// NewSingle creates a Router with a single catch-all route to the given target.
func NewSingle(target string) *Router {
	return &Router{
		routes: []Route{
			{Pattern: "/*", Target: target, Name: "default"},
		},
	}
}

// AddRoute appends a route. Routes are evaluated in order; the first match wins.
func (r *Router) AddRoute(route Route) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = append(r.routes, route)
}

// SetRoutes replaces the entire route table.
func (r *Router) SetRoutes(routes []Route) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = routes
}

// Routes returns a copy of the current route table.
func (r *Router) Routes() []Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Route, len(r.routes))
	copy(result, r.routes)
	return result
}

// Match returns the target for the given path. Returns the first matching
// route's target, or empty string if no route matches.
func (r *Router) Match(path string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, route := range r.routes {
		if matchPattern(route.Pattern, path) {
			return route.Target
		}
	}
	return ""
}

// matchPattern checks if a path matches a pattern.
// Supports:
//   - Exact match: "/api/health" matches "/api/health"
//   - Wildcard prefix: "/api/*" matches "/api/anything/here"
//   - Catch-all: "/*" matches everything
func matchPattern(pattern, path string) bool {
	if pattern == "/*" {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix+"/") || path == prefix
	}
	return pattern == path
}
