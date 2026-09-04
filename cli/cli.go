// Package cli provides the command-line interface for Routa.
package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/7uyash/routa/config"
)

// Command represents a parsed CLI command.
type Command struct {
	Name   string
	Config config.Config
}

// Parse parses command-line arguments and returns a Command.
func Parse() (*Command, error) {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "dev":
		return parseDev()
	case "relay":
		return parseRelay()
	case "version":
		fmt.Println("routa v0.1.0")
		os.Exit(0)
		return nil, nil
	case "help", "--help", "-h":
		printUsage()
		os.Exit(0)
		return nil, nil
	default:
		// Check if first arg is a port number (shorthand: routa 3000)
		if port, err := strconv.Atoi(os.Args[1]); err == nil {
			os.Args = append([]string{os.Args[0], "dev", strconv.Itoa(port)}, os.Args[2:]...)
			return parseDev()
		}
		return nil, fmt.Errorf("unknown command: %s", os.Args[1])
	}
}

func parseDev() (*Command, error) {
	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	fs.StringVar(&cfg.RelayURL, "relay", cfg.RelayURL, "Relay server URL")
	fs.StringVar(&cfg.AuthToken, "token", cfg.AuthToken, "Authentication token")
	fs.IntVar(&cfg.DashboardPort, "dashboard", cfg.DashboardPort, "Dashboard port")
	fs.StringVar(&cfg.TunnelName, "name", cfg.TunnelName, "Tunnel name (subdomain)")
	fs.StringVar(&cfg.BasicAuthUser, "auth-user", cfg.BasicAuthUser, "Basic auth username")
	fs.StringVar(&cfg.BasicAuthPass, "auth-pass", cfg.BasicAuthPass, "Basic auth password")
	fs.StringVar(&cfg.LocalHost, "host", cfg.LocalHost, "Local host to forward to")
	fs.IntVar(&cfg.MaxRecordedEntries, "max-entries", cfg.MaxRecordedEntries, "Max recorded entries")

	// Extract port from positional args.
	args := os.Args[2:]
	if len(args) > 0 {
		if port, err := strconv.Atoi(args[0]); err == nil {
			cfg.LocalPort = port
			args = args[1:]
		}
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if cfg.LocalPort == 0 {
		return nil, fmt.Errorf("port is required: routa dev <port>")
	}

	if err := cfg.Validate("dev"); err != nil {
		return nil, err
	}

	return &Command{Name: "dev", Config: cfg}, nil
}

func parseRelay() (*Command, error) {
	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	fs := flag.NewFlagSet("relay", flag.ExitOnError)
	fs.IntVar(&cfg.RelayPort, "port", cfg.RelayPort, "Relay listen port")
	fs.StringVar(&cfg.RelayHost, "host", cfg.RelayHost, "Relay listen host")
	fs.StringVar(&cfg.BaseDomain, "domain", cfg.BaseDomain, "Base domain for subdomains")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return nil, err
	}

	if err := cfg.Validate("relay"); err != nil {
		return nil, err
	}

	return &Command{Name: "relay", Config: cfg}, nil
}

func printUsage() {
	fmt.Println(`
  ____  ___  _   _ _____ _   
 |  _ \/ _ \| | | |_   _/ \  
 | |_) | | | | | | | |/ _ \ 
 |  _ <| |_| | |_| | / ___ \
 |_| \_\\___/ \___/|_/_/   \_\
                               
  Developer Traffic Gateway — v0.1.0

USAGE:
  routa dev <port>        Start tunnel to local port
  routa relay             Start the relay server
  routa version           Print version

EXAMPLES:
  routa dev 3000                          Tunnel to localhost:3000
  routa dev 8080 --name my-api            Tunnel with custom name
  routa dev 3000 --relay wss://relay.example.com
  routa relay --port 8080 --domain routa.dev

FLAGS (dev):
  --relay <url>           Relay server URL (default: ws://localhost:8080)
  --token <token>         Authentication token
  --dashboard <port>      Dashboard port (default: 4040)
  --name <name>           Tunnel name / subdomain
  --auth-user <user>      Basic auth username for tunnel
  --auth-pass <pass>      Basic auth password for tunnel
  --host <host>           Local host (default: 127.0.0.1)
  --max-entries <n>       Max recorded requests (default: 500)

FLAGS (relay):
  --port <port>           Listen port (default: 8080)
  --host <host>           Listen host (default: 0.0.0.0)
  --domain <domain>       Base domain for subdomains (default: localhost)`)
}
