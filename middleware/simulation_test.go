package middleware

import (
	"sync"
	"testing"
	"time"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/traffic"
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

	res := s.Simulate(traffic.Request{Method: "GET", Path: "/health"})
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

	res := s.Simulate(traffic.Request{Method: "POST", Path: "/api/users"})
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

	res := s.Simulate(traffic.Request{Method: "GET", Path: "/anything"})
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
		res := s.Simulate(traffic.Request{Method: "GET", Path: "/unstable/endpoint"})
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
		res := s.Simulate(traffic.Request{Method: "GET", Path: "/api/test"})
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

	resPost := s.Simulate(traffic.Request{Method: "POST", Path: "/anything"})
	if resPost.Delay == 0 {
		t.Error("POST should match and have delay")
	}

	resGet := s.Simulate(traffic.Request{Method: "GET", Path: "/anything"})
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

	res := s.Simulate(traffic.Request{Method: "GET", Path: "/any"})
	if res.TimeoutMs != 5000 {
		t.Errorf("TimeoutMs = %d, want 5000", res.TimeoutMs)
	}
}

func TestSimulatorConcurrency(t *testing.T) {
	ruleA := []config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("rule-a", "/api/*", "")
			r.DelayMs = 10
			r.JitterMs = 5
			r.ErrorRate = 0.5
			r.ErrorStatus = 503
			return r
		}(),
	}
	ruleB := []config.SimulationConfig{
		func() config.SimulationConfig {
			r := simRule("rule-b", "/api/*", "")
			r.TimeoutMs = 1000
			r.BandwidthBps = 1024
			return r
		}(),
	}

	s := NewSimulator(ruleA)

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Writer updating rules concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		flip := false
		for {
			select {
			case <-done:
				return
			default:
				if flip {
					s.SetRules(ruleA)
				} else {
					s.SetRules(ruleB)
				}
				flip = !flip
			}
		}
	}()

	// Readers calling Simulate and Rules concurrently
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := traffic.Request{Method: "GET", Path: "/api/test"}
			for j := 0; j < 100; j++ {
				_ = s.Rules()
				_ = s.Simulate(req)
			}
		}()
	}

	timeOut := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(done)
		close(timeOut)
	}()
	<-timeOut
	wg.Wait()
}
