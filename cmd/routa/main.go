// Routa — Developer Traffic Gateway
//
// Usage:
//
//	routa dev <port>    Start tunnel to local port
//	routa relay         Start the relay server
//	routa version       Print version
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/7uyash/routa/agent"
	"github.com/7uyash/routa/cli"
	"github.com/7uyash/routa/relay"
)

func main() {
	cmd, err := cli.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGINT / SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n  Shutting down…")
		cancel()
	}()

	switch cmd.Name {
	case "dev":
		runDev(ctx, cmd)
	case "relay":
		runRelay(ctx, cmd)
	}
}

func runDev(ctx context.Context, cmd *cli.Command) {
	cfg := cmd.Config

	// Create display.
	display := cli.NewDisplay(nil, nil, nil)
	display.PrintStartBanner(cfg.LocalTarget(), cfg.DashboardPort)

	// Create and start agent.
	a := agent.New(cfg)

	// Wire up display callbacks.
	display = cli.NewDisplay(
		a.TunnelStats,
		a.RequestCount,
		a.PublicURL,
	)
	_ = display // Display functions are called by the agent via log output

	if err := a.Start(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("[routa] fatal: %v", err)
	}

	a.Stop()
}

func runRelay(ctx context.Context, cmd *cli.Command) {
	cfg := cmd.Config

	fmt.Printf("\n  Routa Relay starting on %s:%d\n", cfg.RelayHost, cfg.RelayPort)
	fmt.Printf("  Base domain: %s\n\n", cfg.BaseDomain)

	srv := relay.NewServer(cfg.BaseDomain)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.RelayHost, cfg.RelayPort),
		Handler: srv,
	}

	go func() {
		<-ctx.Done()
		httpServer.Close()
	}()

	log.Printf("[relay] listening on %s:%d", cfg.RelayHost, cfg.RelayPort)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("[relay] fatal: %v", err)
	}
}
