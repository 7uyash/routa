// Package config handles Routa configuration from CLI flags, environment
// variables, and optional YAML config files.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all configuration for a Routa agent or relay instance.
type Config struct {
	// Agent settings
	LocalPort          int    `json:"local_port" yaml:"local_port"`
	LocalHost          string `json:"local_host" yaml:"local_host"`
	RelayURL           string `json:"relay_url" yaml:"relay_url"`
	AuthToken          string `json:"auth_token" yaml:"auth_token"`
	DashboardPort      int    `json:"dashboard_port" yaml:"dashboard_port"`
	MaxRecordedEntries int    `json:"max_recorded_entries" yaml:"max_recorded_entries"`
	TunnelName         string `json:"tunnel_name" yaml:"tunnel_name"`

	// Basic auth for the public endpoint
	BasicAuthUser string `json:"basic_auth_user" yaml:"basic_auth_user"`
	BasicAuthPass string `json:"basic_auth_pass" yaml:"basic_auth_pass"`

	// Relay settings
	RelayPort    int    `json:"relay_port" yaml:"relay_port"`
	RelayHost    string `json:"relay_host" yaml:"relay_host"`
	BaseDomain   string `json:"base_domain" yaml:"base_domain"`

	// Data directory for sessions and storage
	DataDir string `json:"data_dir" yaml:"data_dir"`

	// Parsed routa.yaml project config (nil if no file found).
	ProjectCfg *ProjectConfig `json:"-" yaml:"-"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		LocalHost:          "127.0.0.1",
		RelayURL:           "",
		DashboardPort:      4040,
		MaxRecordedEntries: 500,
		RelayPort:          8080,
		RelayHost:          "0.0.0.0",
		BaseDomain:         "localhost",
		DataDir:            filepath.Join(homeDir, ".routa"),
	}
}

// Validate checks the config for required fields and invalid values.
func (c *Config) Validate(mode string) error {
	switch mode {
	case "dev":
		if c.LocalPort <= 0 || c.LocalPort > 65535 {
			return fmt.Errorf("local port must be between 1 and 65535, got %d", c.LocalPort)
		}
		if c.RelayURL == "" {
			return fmt.Errorf("relay URL is required")
		}
		if c.DashboardPort <= 0 || c.DashboardPort > 65535 {
			return fmt.Errorf("dashboard port must be between 1 and 65535, got %d", c.DashboardPort)
		}
	case "relay":
		if c.RelayPort <= 0 || c.RelayPort > 65535 {
			return fmt.Errorf("relay port must be between 1 and 65535, got %d", c.RelayPort)
		}
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
	return nil
}

// LocalTarget returns the full local address string (host:port).
func (c *Config) LocalTarget() string {
	return fmt.Sprintf("http://%s:%d", c.LocalHost, c.LocalPort)
}

// SessionsDir returns the path where sessions are stored.
func (c *Config) SessionsDir() string {
	return filepath.Join(c.DataDir, "sessions")
}

// LoadFromEnv overrides config values from environment variables.
func (c *Config) LoadFromEnv() {
	if v := os.Getenv("ROUTA_LOCAL_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.LocalPort = p
		}
	}
	if v := os.Getenv("ROUTA_RELAY_URL"); v != "" {
		c.RelayURL = v
	}
	if v := os.Getenv("ROUTA_AUTH_TOKEN"); v != "" {
		c.AuthToken = v
	}
	if v := os.Getenv("ROUTA_DASHBOARD_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.DashboardPort = p
		}
	}
	if v := os.Getenv("ROUTA_BASIC_AUTH_USER"); v != "" {
		c.BasicAuthUser = v
	}
	if v := os.Getenv("ROUTA_BASIC_AUTH_PASS"); v != "" {
		c.BasicAuthPass = v
	}
	if v := os.Getenv("ROUTA_TUNNEL_NAME"); v != "" {
		c.TunnelName = v
	}
	if v := os.Getenv("ROUTA_BASE_DOMAIN"); v != "" {
		c.BaseDomain = v
	}
	if v := os.Getenv("ROUTA_RELAY_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.RelayPort = p
		}
	}
	if v := os.Getenv("ROUTA_DATA_DIR"); v != "" {
		c.DataDir = v
	}
}
