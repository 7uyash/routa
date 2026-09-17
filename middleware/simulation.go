package middleware

import (
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/traffic"
)

// Simulator applies network/failure simulation rules to a request before
// it is forwarded to the local service. Call Simulate() right before Forward().
type Simulator struct {
	mu    sync.RWMutex
	rules []config.SimulationConfig
	rngMu sync.Mutex
	rng   *rand.Rand
}

// NewSimulator creates a Simulator with the given rules.
func NewSimulator(rules []config.SimulationConfig) *Simulator {
	cp := make([]config.SimulationConfig, len(rules))
	copy(cp, rules)
	return &Simulator{
		rules: cp,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetRules replaces the rule set at runtime.
func (s *Simulator) SetRules(rules []config.SimulationConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]config.SimulationConfig, len(rules))
	copy(cp, rules)
	s.rules = cp
}

// Rules returns a copy of the current rule set.
func (s *Simulator) Rules() []config.SimulationConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]config.SimulationConfig, len(s.rules))
	copy(cp, s.rules)
	return cp
}

// SimResult describes what the simulator decided for this request.
type SimResult struct {
	// ShouldDrop means: don't forward, don't respond (simulate dropped connection).
	ShouldDrop bool
	// InjectedStatus means: return this status without forwarding.
	InjectedStatus int
	// Delay is how long to sleep before forwarding.
	Delay time.Duration
	// TimeoutMs limits how long the forward call may take.
	TimeoutMs int
	// BandwidthBps limits the response read rate (0 = unlimited).
	BandwidthBps int
	// MatchedRule is the name of the first rule that applied.
	MatchedRule string
}

// Simulate evaluates all matching rules for the given request and returns
// a SimResult. If multiple rules match, the first one wins (except delay
// which accumulates).
func (s *Simulator) Simulate(req traffic.Request) SimResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := SimResult{}

	for _, rule := range s.rules {
		if !simMatchesRule(rule.Match, req.Method, req.Path) {
			continue
		}
		if result.MatchedRule == "" {
			result.MatchedRule = rule.Name
		}

		// Drop takes absolute priority.
		if rule.Drop {
			result.ShouldDrop = true
			return result
		}

		// Error injection.
		if rule.ErrorRate > 0 {
			s.rngMu.Lock()
			f := s.rng.Float64()
			s.rngMu.Unlock()
			if f < rule.ErrorRate {
				status := rule.ErrorStatus
				if status == 0 {
					status = 503
				}
				result.InjectedStatus = status
				return result
			}
		}

		// Latency (accumulates across matching rules).
		if rule.DelayMs > 0 {
			jitter := 0
			if rule.JitterMs > 0 {
				s.rngMu.Lock()
				jitter = s.rng.Intn(rule.JitterMs*2+1) - rule.JitterMs
				s.rngMu.Unlock()
			}
			delayMs := rule.DelayMs + jitter
			if delayMs < 0 {
				delayMs = 0
			}
			result.Delay += time.Duration(delayMs) * time.Millisecond
		}

		// Timeout.
		if rule.TimeoutMs > 0 && result.TimeoutMs == 0 {
			result.TimeoutMs = rule.TimeoutMs
		}

		// Bandwidth throttle.
		if rule.BandwidthBps > 0 && result.BandwidthBps == 0 {
			result.BandwidthBps = rule.BandwidthBps
		}
	}

	return result
}

// ApplyDelay sleeps for the duration in SimResult.Delay.
func ApplyDelay(res SimResult) {
	if res.Delay > 0 {
		time.Sleep(res.Delay)
	}
}

// simMatchesRule checks if a method+path matches a SimulationConfig match block.
func simMatchesRule(match config.MatchConfig, method, path string) bool {
	if match.Method != "" && !strings.EqualFold(match.Method, method) {
		return false
	}
	if match.Path != "" {
		if strings.HasSuffix(match.Path, "*") {
			if !strings.HasPrefix(path, strings.TrimSuffix(match.Path, "*")) {
				return false
			}
		} else {
			if path != match.Path {
				return false
			}
		}
	}
	return true
}
