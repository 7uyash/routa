package agent

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/7uyash/routa/config"
)

// These tests drive the real ServeHTTP pipeline (router -> mock lab -> mutation
// -> simulation -> forward -> record) over httptest, and exist to pin down
// issue #9: one pattern has to mean the same thing at every stage of the
// pipeline. The package-level matchers in traffic, router, middleware and mock
// are unit tested on their own; what these catch is the wiring between them, and
// in particular the bare "/api" prefix that used to fall straight through the
// mutation and mock stages.

// newStubTarget starts an upstream that echoes every X-* request header back as
// a response header. Traffic mutations rewrite the outgoing request, so the
// only way to observe one is from the upstream's side.
func newStubTarget(t *testing.T, body string) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, vals := range r.Header {
			if strings.HasPrefix(k, "X-") {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse stub target url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse stub target port: %v", err)
	}
	return port
}

// newAgentBehind starts the real agent on a test server and returns its URL.
func newAgentBehind(t *testing.T, targetPort int, proj *config.ProjectConfig) string {
	t.Helper()
	a := New(config.Config{
		LocalHost:  "127.0.0.1",
		LocalPort:  targetPort,
		ProjectCfg: proj,
	})
	srv := httptest.NewServer(http.HandlerFunc(a.ServeHTTP))
	t.Cleanup(srv.Close)
	return srv.URL
}

func proxyGet(t *testing.T, base, path string) (int, http.Header) {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header
}

// TestMutationStageMatchesSegmentWildcard is the core of the issue: a "/api/*"
// mutation has to fire for "/api" itself, not only for "/api/something".
func TestMutationStageMatchesSegmentWildcard(t *testing.T) {
	port := newStubTarget(t, "upstream")
	base := newAgentBehind(t, port, &config.ProjectConfig{
		Mutations: []config.MutationConfig{{
			Name:    "probe",
			Match:   config.MatchConfig{Path: "/api/*"},
			Request: config.RequestMutation{SetHeaders: map[string]string{"X-Probe": "mutation"}},
		}},
	})

	for _, path := range []string{"/api", "/api/users", "/api/v1/users/1"} {
		_, hdr := proxyGet(t, base, path)
		if got := hdr.Get("X-Probe"); got != "mutation" {
			t.Errorf("GET %s: X-Probe = %q, want %q (mutation on /api/* should fire)", path, got, "mutation")
		}
	}

	for _, path := range []string{"/apifoo", "/other", "/"} {
		_, hdr := proxyGet(t, base, path)
		if got := hdr.Get("X-Probe"); got != "" {
			t.Errorf("GET %s: X-Probe = %q, want empty (the segment boundary should hold)", path, got)
		}
	}
}

// TestSimulationStageMatchesSamePattern proves the simulation stage agrees with
// the mutation stage about the same pattern.
func TestSimulationStageMatchesSamePattern(t *testing.T) {
	port := newStubTarget(t, "upstream")
	base := newAgentBehind(t, port, &config.ProjectConfig{
		Simulations: []config.SimulationConfig{{
			Name:        "probe",
			Match:       config.MatchConfig{Path: "/api/*"},
			ErrorRate:   1.0,
			ErrorStatus: 503,
		}},
	})

	for _, path := range []string{"/api", "/api/users"} {
		if status, _ := proxyGet(t, base, path); status != 503 {
			t.Errorf("GET %s: status = %d, want 503 (simulation on /api/* should fire)", path, status)
		}
	}

	for _, path := range []string{"/apifoo", "/other"} {
		if status, _ := proxyGet(t, base, path); status != 200 {
			t.Errorf("GET %s: status = %d, want 200 (the segment boundary should hold)", path, status)
		}
	}
}

// TestRouterStageAcceptsPreviouslyDeadPatterns covers the router side. "*" and
// "/prefix*" used to be compared literally, so a route with either of those
// patterns matched nothing and every request fell back to the startup target.
func TestRouterStageAcceptsPreviouslyDeadPatterns(t *testing.T) {
	port := newStubTarget(t, "upstream")
	// Both routes point at a port nothing is listening on, so a request that a
	// route claims comes back as 502. Before the fix no route matched and the
	// request fell back to the healthy startup target, producing 200.
	base := newAgentBehind(t, port, &config.ProjectConfig{
		Routes: []config.RouteConfig{
			{Pattern: "/api*", Target: "http://127.0.0.1:1", Name: "dead-target-prefix"},
			{Pattern: "*", Target: "http://127.0.0.1:1", Name: "dead-target-catchall"},
		},
	})

	for _, path := range []string{"/api/users", "/apifoo"} {
		if status, _ := proxyGet(t, base, path); status != 502 {
			t.Errorf("GET %s: status = %d, want 502 (the /api* route should claim it)", path, status)
		}
	}
	for _, path := range []string{"/anything", "/", "/deep/path"} {
		if status, _ := proxyGet(t, base, path); status != 502 {
			t.Errorf("GET %s: status = %d, want 502 (the * route should claim it)", path, status)
		}
	}
}

// TestMethodMatchingAgreesAcrossStages covers method matching through the
// pipeline: a lowercase "get" pattern and an empty pattern both mean "any".
func TestMethodMatchingAgreesAcrossStages(t *testing.T) {
	port := newStubTarget(t, "upstream")
	base := newAgentBehind(t, port, &config.ProjectConfig{
		Mutations: []config.MutationConfig{
			{
				Name:    "lowercase",
				Match:   config.MatchConfig{Method: "get", Path: "/lower/*"},
				Request: config.RequestMutation{SetHeaders: map[string]string{"X-Lower": "1"}},
			},
			{
				Name:    "any-method",
				Match:   config.MatchConfig{Path: "/any/*"},
				Request: config.RequestMutation{SetHeaders: map[string]string{"X-Any": "1"}},
			},
		},
	})

	_, hdr := proxyGet(t, base, "/lower/thing")
	if got := hdr.Get("X-Lower"); got != "1" {
		t.Errorf("GET /lower/thing: X-Lower = %q, want 1 (a lowercase 'get' should match GET)", got)
	}

	// An empty method means "any method", so a POST has to hit the rule too.
	resp, err := http.Post(base+"/any/thing", "text/plain", nil)
	if err != nil {
		t.Fatalf("POST /any/thing: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Any"); got != "1" {
		t.Errorf("POST /any/thing: X-Any = %q, want 1 (an empty method should match any)", got)
	}
	if got := resp.Header.Get("X-Lower"); got != "" {
		t.Errorf("POST /any/thing: X-Lower = %q, want empty (the 'get' rule should not match POST)", got)
	}
}
