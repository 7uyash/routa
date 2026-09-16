package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/discovery"
	"github.com/charmbracelet/huh"
)

func runInteractiveMainMenu() (*Command, error) {
	fmt.Println("Routa — Developer Traffic Gateway\n")

	var action string
	err := huh.NewSelect[string]().
		Title("What would you like to do?").
		Options(
			huh.NewOption("Expose a local service", "dev"),
			huh.NewOption("Start a relay server", "relay"),
			huh.NewOption("Exit", "exit"),
		).
		Value(&action).
		Run()

	if err != nil {
		return nil, err
	}

	switch action {
	case "dev":
		return runInteractiveDev()
	case "relay":
		return runInteractiveRelay()
	case "exit":
		os.Exit(0)
	}
	return nil, nil
}

func runInteractiveDev() (*Command, error) {
	fmt.Println("\nScanning for local services...")
	scanner := discovery.NewScanner()
	services := scanner.Scan()

	options := make([]huh.Option[string], 0, len(services)+1)
	for _, svc := range services {
		label := fmt.Sprintf("%-15s %s", fmt.Sprintf("localhost:%d", svc.Port), svc.FriendlyName)
		options = append(options, huh.NewOption(label, strconv.Itoa(svc.Port)))
	}
	options = append(options, huh.NewOption("Enter port manually", "manual"))

	var selectedPortStr string
	err := huh.NewSelect[string]().
		Title("Select a local service:").
		Options(options...).
		Value(&selectedPortStr).
		Run()

	if err != nil {
		return nil, err
	}

	var port int
	if selectedPortStr == "manual" {
		var manualPortStr string
		err = huh.NewInput().
			Title("Enter port number:").
			Value(&manualPortStr).
			Run()
		if err != nil {
			return nil, err
		}
		port, err = strconv.Atoi(manualPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", manualPortStr)
		}
	} else {
		port, _ = strconv.Atoi(selectedPortStr)
	}

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()
	cfg.LocalPort = port

	fmt.Printf("\nSelected: localhost:%d\n\n", port)

	var authType string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Tunnel name (optional)").
				Value(&cfg.TunnelName),
			huh.NewInput().
				Title("Relay server").
				Value(&cfg.RelayURL),
			huh.NewSelect[string]().
				Title("Authentication").
				Options(
					huh.NewOption("none", "none"),
					huh.NewOption("token", "token"),
					huh.NewOption("basic", "basic"),
				).
				Value(&authType),
		),
	).Run()

	if err != nil {
		return nil, err
	}

	if authType == "token" {
		huh.NewInput().Title("Auth Token").Value(&cfg.AuthToken).Run()
	} else if authType == "basic" {
		huh.NewInput().Title("Username").Value(&cfg.BasicAuthUser).Run()
		huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&cfg.BasicAuthPass).Run()
	}

	var confirm bool
	err = huh.NewConfirm().
		Title("Start tunnel?").
		Value(&confirm).
		Run()

	if err != nil {
		return nil, err
	}
	if !confirm {
		fmt.Println("Aborted.")
		os.Exit(0)
	}

	if err := cfg.Validate("dev"); err != nil {
		return nil, err
	}

	// Print equivalent command
	cmdStr := fmt.Sprintf("routa dev %d", cfg.LocalPort)
	if cfg.TunnelName != "" {
		cmdStr += fmt.Sprintf(" --name %s", cfg.TunnelName)
	}
	if cfg.RelayURL != config.DefaultConfig().RelayURL {
		cmdStr += fmt.Sprintf(" --relay %s", cfg.RelayURL)
	}
	fmt.Printf("\n%s\n\n", cmdStr)

	return &Command{Name: "dev", Config: cfg}, nil
}

func runInteractiveRelay() (*Command, error) {
	fmt.Println("\nConfigure Routa Relay\n")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	var portStr string = strconv.Itoa(cfg.RelayPort)

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Listen host:").
				Options(
					huh.NewOption("0.0.0.0", "0.0.0.0"),
					huh.NewOption("127.0.0.1", "127.0.0.1"),
					huh.NewOption("Custom", "custom"),
				).
				Value(&cfg.RelayHost),
			huh.NewInput().
				Title("Listen port:").
				Value(&portStr),
			huh.NewInput().
				Title("Base domain:").
				Value(&cfg.BaseDomain),
		),
	).Run()

	if err != nil {
		return nil, err
	}

	if cfg.RelayHost == "custom" {
		huh.NewInput().Title("Enter custom host:").Value(&cfg.RelayHost).Run()
	}

	cfg.RelayPort, _ = strconv.Atoi(portStr)

	var confirm bool
	err = huh.NewConfirm().
		Title("Start relay?").
		Value(&confirm).
		Run()

	if err != nil {
		return nil, err
	}
	if !confirm {
		fmt.Println("Aborted.")
		os.Exit(0)
	}

	if err := cfg.Validate("relay"); err != nil {
		return nil, err
	}

	return &Command{Name: "relay", Config: cfg}, nil
}
