package middleware

import (
	"testing"
	"time"

	"github.com/7uyash/routa/config"
)

func simRule(name, path, method string) config.SimulationConfig {
	return config.SimulationConfig{
		Name:  name,
		Match: config.MatchConfig{Path: path, Method: method},
	}
}

func TestSimulatorNoMatch(t *testing.T) {
	s := NewSimulator([]config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("api-delay", "/api/*", "")
			r.DelayMs = 200
			return r
		}(),
	})

	res := s.Simulate("GET", "/health")
	if res.Delay != 0 {
		t.Errorf("Delay should be 0 for non-matching path, got %v", res.Delay)
	}
	if res.MatchedRule != "" {
		t.Errorf("MatchedRule should be empty, got %q", res.MatchedRule)
	}
}

func TestSimulatorLatencyInjection(t *testing.T) {
	s := NewSimulator([]config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("slow-api", "/api/*", "")
			r.DelayMs = 150
			return r
		}(),
	})

	res := s.Simulate("POST", "/api/users")
	if res.Delay < 100*time.Millisecond {
		t.Errorf("Delay too low: %v, want >= 100ms", res.Delay)
	}
	if res.MatchedRule != "slow-api" {
		t.Errorf("MatchedRule = %q, want slow-api", res.MatchedRule)
	}
}

func TestSimulatorDrop(t *testing.T) {
	s := NewSimulator([]config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("drop-all", "", "")
			r.Drop = true
			return r
		}(),
	})

	res := s.Simulate("GET", "/anything")
	if !res.ShouldDrop {
		t.Error("expected ShouldDrop = true")
	}
}

func TestSimulatorErrorInjection100Pct(t *testing.T) {
	s := NewSimulator([]config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("force-error", "/unstable/*", "")
			r.ErrorRate = 1.0
			r.ErrorStatus = 503
			return r
		}(),
	})

	// With 100% error rate, should always inject an error
	for i := 0; i < 5; i++ {
		res := s.Simulate("GET", "/unstable/endpoint")
		if res.InjectedStatus != 503 {
			t.Errorf("iteration %d: InjectedStatus = %d, want 503", i, res.InjectedStatus)
		}
	}
}

func TestSimulatorErrorInjection0Pct(t *testing.T) {
	s := NewSimulator([]config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("zero-error", "/api/*", "")
			r.ErrorRate = 0
			r.ErrorStatus = 500
			return r
		}(),
	})

	// 0% rate should never inject
	for i := 0; i < 10; i++ {
		res := s.Simulate("GET", "/api/test")
		if res.InjectedStatus != 0 {
			t.Errorf("0%% rate triggered injection on iteration %d", i)
		}
	}
}

func TestSimulatorMethodMatch(t *testing.T) {
	s := NewSimulator([]config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("post-delay", "", "POST")
			r.DelayMs = 200
			return r
		}(),
	})

	resPost := s.Simulate("POST", "/anything")
	if resPost.Delay == 0 {
		t.Error("POST should match and have delay")
	}

	resGet := s.Simulate("GET", "/anything")
	if resGet.Delay != 0 {
		t.Error("GET should not match POST rule")
	}
}

func TestSimulatorTimeout(t *testing.T) {
	s := NewSimulator([]config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("timeout-rule", "/*", "")
			r.TimeoutMs = 5000
			return r
		}(),
	})

	res := s.Simulate("GET", "/any")
	if res.TimeoutMs != 5000 {
		t.Errorf("TimeoutMs = %d, want 5000", res.TimeoutMs)
	}
}
