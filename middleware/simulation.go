package middleware

import (
	"math/rand"
	"strings"
	"time"

	"github.com/7uyash/routa/config"
)

// Simulator applies network/failure simulation rules to a request before
// it is forwarded to the local service. Call Simulate() right before Forward().
type Simulator struct {
	rules []config.SimulationConfig
	rng   *rand.Rand
}

// NewSimulator creates a Simulator with the given rules.
func NewSimulator(rules []config.SimulationConfig) *Simulator {
	return &Simulator{
		rules: rules,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SetRules replaces the rule set at runtime.
func (s *Simulator) SetRules(rules []config.SimulationConfig) {
	s.rules = rules
}

// Rules returns a copy of the current rule set.
func (s *Simulator) Rules() []config.SimulationConfig {
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
func (s *Simulator) Simulate(method, path string) SimResult {
	result := SimResult{}

	for _, rule := range s.rules {
		if !simMatchesRule(rule.Match, method, path) {
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
		if rule.ErrorRate > 0 && s.rng.Float64() < rule.ErrorRate {
			status := rule.ErrorStatus
			if status == 0 {
				status = 503
			}
			result.InjectedStatus = status
			return result
		}

		// Latency (accumulates across matching rules).
		if rule.DelayMs > 0 {
			jitter := 0
			if rule.JitterMs > 0 {
				jitter = s.rng.Intn(rule.JitterMs*2+1) - rule.JitterMs
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
