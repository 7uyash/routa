package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ProjectConfig is the on-disk format of routa.yaml placed in the project root.
// It overrides CLI flags for everything defined within.
type ProjectConfig struct {
	// Tunnel settings
	Tunnel TunnelConfig `yaml:"tunnel"`

	// Multi-service routes
	Routes []RouteConfig `yaml:"routes"`

	// Traffic mutation rules (applied in order)
	Mutations []MutationConfig `yaml:"mutations"`

	// Network/failure simulation rules
	Simulations []SimulationConfig `yaml:"simulations"`

	// Shadow traffic targets
	Shadow ShadowConfig `yaml:"shadow"`

	// Recording / redaction settings
	Recording RecordingConfig `yaml:"recording"`
}

// TunnelConfig holds tunnel-specific settings from routa.yaml.
type TunnelConfig struct {
	Port          int    `yaml:"port"`
	RelayURL      string `yaml:"relay_url"`
	Name          string `yaml:"name"`
	DashboardPort int    `yaml:"dashboard_port"`
	AuthToken     string `yaml:"auth_token"`
	BasicAuthUser string `yaml:"basic_auth_user"`
	BasicAuthPass string `yaml:"basic_auth_pass"`
}

// RouteConfig maps a path pattern to a local target.
type RouteConfig struct {
	Pattern string `yaml:"pattern"`
	Target  string `yaml:"target"`
	Name    string `yaml:"name"`
}

// MutationConfig describes a traffic mutation rule.
type MutationConfig struct {
	Name    string            `yaml:"name"`
	Match   MatchConfig       `yaml:"match"`
	Request RequestMutation   `yaml:"request"`
	Response ResponseMutation `yaml:"response"`
}

// MatchConfig defines conditions for when a rule applies.
type MatchConfig struct {
	// Path prefix or glob. Empty = match all.
	Path   string `yaml:"path"`
	Method string `yaml:"method"`
}

// RequestMutation describes mutations to the outgoing request.
type RequestMutation struct {
	// Add/override headers. Key→Value.
	SetHeaders    map[string]string `yaml:"set_headers"`
	RemoveHeaders []string          `yaml:"remove_headers"`
	// Rewrite the path: strip prefix, or regex-replace.
	StripPathPrefix string `yaml:"strip_path_prefix"`
	ReplacePath     string `yaml:"replace_path"` // literal new path
	// Set/override query parameters.
	SetQuery    map[string]string `yaml:"set_query"`
	RemoveQuery []string          `yaml:"remove_query"`
	// JSON body field mutations: dot-notation path → new value (as JSON string).
	SetBodyFields map[string]string `yaml:"set_body_fields"`
}

// ResponseMutation describes mutations to the incoming response.
type ResponseMutation struct {
	// Add/override response headers.
	SetHeaders    map[string]string `yaml:"set_headers"`
	RemoveHeaders []string          `yaml:"remove_headers"`
	// Override the status code.
	ForceStatus int `yaml:"force_status"`
	// Return a mock response instead of forwarding at all.
	MockStatus  int    `yaml:"mock_status"`
	MockBody    string `yaml:"mock_body"`
	MockHeaders map[string]string `yaml:"mock_headers"`
}

// SimulationConfig describes network/failure simulation for a route.
type SimulationConfig struct {
	Name  string      `yaml:"name"`
	Match MatchConfig `yaml:"match"`

	// Latency injection (milliseconds).
	DelayMs  int `yaml:"delay_ms"`
	JitterMs int `yaml:"jitter_ms"` // random ±jitter added on top of delay

	// Bandwidth throttle in bytes/second. 0 = unlimited.
	BandwidthBps int `yaml:"bandwidth_bps"`

	// Error injection: return this HTTP status at the given rate (0.0–1.0).
	ErrorRate   float64 `yaml:"error_rate"`
	ErrorStatus int     `yaml:"error_status"` // default 503

	// Timeout: kill the forwarded connection after this many ms.
	TimeoutMs int `yaml:"timeout_ms"`

	// Drop: don't send any response (simulate connection drop).
	Drop bool `yaml:"drop"`
}

// ShadowConfig names additional targets to mirror traffic to.
type ShadowConfig struct {
	Enabled bool     `yaml:"enabled"`
	Targets []string `yaml:"targets"` // e.g. ["http://localhost:3001"]
}

// RecordingConfig controls what gets captured and redacted.
type RecordingConfig struct {
	Enabled            bool     `yaml:"enabled"`
	MaxEntries         int      `yaml:"max_entries"`
	RedactHeaders      []string `yaml:"redact_headers"`       // e.g. ["Authorization", "Cookie"]
	RedactBodyFields   []string `yaml:"redact_body_fields"`   // dot-path fields to blank out
	ExcludePaths       []string `yaml:"exclude_paths"`        // paths never recorded
}

// LoadProjectConfig reads routa.yaml from the given directory.
// Returns nil if no file exists (not an error).
func LoadProjectConfig(dir string) (*ProjectConfig, error) {
	path := dir + "/routa.yaml"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read routa.yaml: %w", err)
	}

	var pc ProjectConfig
	if err := yaml.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("parse routa.yaml: %w", err)
	}
	return &pc, nil
}

// Apply merges a ProjectConfig into a Config, letting routa.yaml override CLI flags.
func (c *Config) Apply(pc *ProjectConfig) {
	if pc == nil {
		return
	}
	if pc.Tunnel.Port != 0 {
		c.LocalPort = pc.Tunnel.Port
	}
	if pc.Tunnel.RelayURL != "" {
		c.RelayURL = pc.Tunnel.RelayURL
	}
	if pc.Tunnel.Name != "" {
		c.TunnelName = pc.Tunnel.Name
	}
	if pc.Tunnel.DashboardPort != 0 {
		c.DashboardPort = pc.Tunnel.DashboardPort
	}
	if pc.Tunnel.AuthToken != "" {
		c.AuthToken = pc.Tunnel.AuthToken
	}
	if pc.Tunnel.BasicAuthUser != "" {
		c.BasicAuthUser = pc.Tunnel.BasicAuthUser
	}
	if pc.Tunnel.BasicAuthPass != "" {
		c.BasicAuthPass = pc.Tunnel.BasicAuthPass
	}
	if pc.Recording.MaxEntries != 0 {
		c.MaxRecordedEntries = pc.Recording.MaxEntries
	}
	// Routes, Mutations, Simulations and Shadow are stored directly.
	c.ProjectCfg = pc
}
