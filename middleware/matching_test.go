package middleware

import (
	"testing"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/router"
	"github.com/7uyash/routa/traffic"
)

// matchingCases are the (pattern, path) pairs that mutations, simulation, mocks
// and routes previously disagreed on. See traffic.MatchPath for the rules.
var matchingCases = []struct {
	name    string
	pattern string
	path    string
	want    bool
}{
	{"exact match", "/health", "/health", true},
	{"exact match does not spill over", "/health", "/healthy", false},
	{"segment wildcard matches the prefix", "/api/*", "/api", true},
	{"segment wildcard matches below", "/api/*", "/api/users", true},
	{"segment wildcard respects the boundary", "/api/*", "/apifoo", false},
	{"string prefix wildcard", "/api*", "/apifoo", true},
	{"catch-all", "/*", "/anything", true},
	{"bare star", "*", "/anything", true},
	{"empty pattern matches all", "", "/anything", true},
	{"unrelated path", "/api/*", "/other", false},
}

func TestMutatorUsesCanonicalPathMatching(t *testing.T) {
	for _, tt := range matchingCases {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMutator([]config.MutationConfig{{
				Name:  "probe",
				Match: config.MatchConfig{Path: tt.pattern},
				Request: config.RequestMutation{
					SetHeaders: map[string]string{"X-Probe": "1"},
				},
			}})

			req := traffic.Request{Method: "GET", Path: tt.path, Headers: map[string][]string{}}
			got := m.ApplyToRequest(req).Request.Headers.Get("X-Probe") == "1"

			if got != tt.want {
				t.Errorf("mutation with pattern %q fired for %q = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestSimulatorUsesCanonicalPathMatching(t *testing.T) {
	for _, tt := range matchingCases {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSimulator([]config.SimulationConfig{{
				Name:        "probe",
				Match:       config.MatchConfig{Path: tt.pattern},
				ErrorRate:   1.0,
				ErrorStatus: 503,
			}})

			req := traffic.Request{Method: "GET", Path: tt.path}
			got := s.Simulate(req).InjectedStatus != 0

			if got != tt.want {
				t.Errorf("simulation with pattern %q fired for %q = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestMutatorAndSimulatorUseCanonicalMethodMatching(t *testing.T) {
	methodCases := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{"exact method", "GET", "/api/*", true},
		{"lowercase method pattern still matches", "get", "/api/*", true},
		{"different method", "POST", "/api/*", false},
		{"star matches any method", "*", "/api/*", true},
		{"empty method matches any method", "", "/api/*", true},
	}

	for _, tt := range methodCases {
		t.Run(tt.name, func(t *testing.T) {
			match := config.MatchConfig{Method: tt.method, Path: tt.path}
			req := traffic.Request{Method: "GET", Path: "/api/users", Headers: map[string][]string{}}

			m := NewMutator([]config.MutationConfig{{
				Name:    "probe",
				Match:   match,
				Request: config.RequestMutation{SetHeaders: map[string]string{"X-Probe": "1"}},
			}})
			mutationFired := m.ApplyToRequest(req).Request.Headers.Get("X-Probe") == "1"

			s := NewSimulator([]config.SimulationConfig{{
				Name:        "probe",
				Match:       match,
				ErrorRate:   1.0,
				ErrorStatus: 503,
			}})
			simulationFired := s.Simulate(req).InjectedStatus != 0

			if mutationFired != tt.want {
				t.Errorf("mutation with method %q fired = %v, want %v", tt.method, mutationFired, tt.want)
			}
			if simulationFired != tt.want {
				t.Errorf("simulation with method %q fired = %v, want %v", tt.method, simulationFired, tt.want)
			}
		})
	}
}

// TestFeaturesAgreeOnEveryPattern is the regression guard for issue #9: the
// same pattern must mean the same thing in every feature. The router is
// included because it used to be the odd one out for "*" and "/prefix*".
func TestFeaturesAgreeOnEveryPattern(t *testing.T) {
	for _, tt := range matchingCases {
		t.Run(tt.name, func(t *testing.T) {
			r := router.New(router.Route{Pattern: tt.pattern, Target: "http://hit"})
			routeFired := r.Match(tt.path) != ""

			m := NewMutator([]config.MutationConfig{{
				Name:    "probe",
				Match:   config.MatchConfig{Path: tt.pattern},
				Request: config.RequestMutation{SetHeaders: map[string]string{"X-Probe": "1"}},
			}})
			req := traffic.Request{Method: "GET", Path: tt.path, Headers: map[string][]string{}}
			mutationFired := m.ApplyToRequest(req).Request.Headers.Get("X-Probe") == "1"

			s := NewSimulator([]config.SimulationConfig{{
				Name:        "probe",
				Match:       config.MatchConfig{Path: tt.pattern},
				ErrorRate:   1.0,
				ErrorStatus: 503,
			}})
			simulationFired := s.Simulate(req).InjectedStatus != 0

			if routeFired != tt.want {
				t.Errorf("router with pattern %q matched %q = %v, want %v", tt.pattern, tt.path, routeFired, tt.want)
			}
			if mutationFired != routeFired {
				t.Errorf("pattern %q: mutation fired = %v but router matched = %v", tt.pattern, mutationFired, routeFired)
			}
			if simulationFired != routeFired {
				t.Errorf("pattern %q: simulation fired = %v but router matched = %v", tt.pattern, simulationFired, routeFired)
			}
		})
	}
}
