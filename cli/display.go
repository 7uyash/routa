package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/7uyash/routa/tunnel"
)

// Display manages the terminal output for the running agent.
type Display struct {
	tunnelStats func() tunnel.Stats
	requestCount func() int
	publicURL    func() string
}

// NewDisplay creates a Display with the given stat providers.
func NewDisplay(stats func() tunnel.Stats, reqCount func() int, pubURL func() string) *Display {
	return &Display{
		tunnelStats:  stats,
		requestCount: reqCount,
		publicURL:    pubURL,
	}
}

// PrintStartBanner prints the initial startup information.
func (d *Display) PrintStartBanner(localTarget string, dashboardPort int) {
	fmt.Println()
	fmt.Println("  \033[38;5;99m╔═══════════════════════════════════════════════╗\033[0m")
	fmt.Println("  \033[38;5;99m║\033[0m           \033[1;38;5;141mRouta\033[0m — Traffic Gateway             \033[38;5;99m║\033[0m")
	fmt.Println("  \033[38;5;99m╚═══════════════════════════════════════════════╝\033[0m")
	fmt.Println()
	fmt.Printf("  \033[38;5;245m%-14s\033[0m %s\n", "Local target:", localTarget)
	fmt.Printf("  \033[38;5;245m%-14s\033[0m http://localhost:%d\n", "Dashboard:", dashboardPort)
	fmt.Println()
}

// PrintConnected prints the connection success info.
func (d *Display) PrintConnected(publicURL, subdomain string) {
	fmt.Printf("  \033[38;5;245m%-14s\033[0m \033[1;38;5;114m%s\033[0m\n", "Public URL:", publicURL)
	fmt.Printf("  \033[38;5;245m%-14s\033[0m %s\n", "Subdomain:", subdomain)
	fmt.Println()
	fmt.Println("  \033[38;5;245m──────────────────────────────────────────────\033[0m")
	fmt.Println("  \033[38;5;245mWaiting for requests…\033[0m")
	fmt.Println()
}

// PrintDisconnected prints a disconnection notice.
func (d *Display) PrintDisconnected() {
	fmt.Println("\n  \033[38;5;196m⚠  Tunnel disconnected — reconnecting…\033[0m")
}

// PrintReconnected prints a reconnection notice.
func (d *Display) PrintReconnected() {
	fmt.Println("  \033[38;5;114m✓  Tunnel reconnected\033[0m")
}

// PrintRequest prints a single request log line.
func (d *Display) PrintRequest(method, path string, status int, duration time.Duration) {
	methodColor := methodColorCode(method)
	statusColor := statusColorCode(status)

	fmt.Printf("  %s%-7s\033[0m %s → %s%d\033[0m \033[38;5;245m(%s)\033[0m\n",
		methodColor, method,
		truncatePath(path, 50),
		statusColor, status,
		formatDuration(duration))
}

// --- Helpers ---

func methodColorCode(method string) string {
	switch strings.ToUpper(method) {
	case "GET":
		return "\033[38;5;114m" // green
	case "POST":
		return "\033[38;5;75m" // blue
	case "PUT":
		return "\033[38;5;221m" // yellow
	case "PATCH":
		return "\033[38;5;141m" // purple
	case "DELETE":
		return "\033[38;5;196m" // red
	default:
		return "\033[38;5;245m" // gray
	}
}

func statusColorCode(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "\033[38;5;114m" // green
	case status >= 300 && status < 400:
		return "\033[38;5;75m" // blue
	case status >= 400 && status < 500:
		return "\033[38;5;221m" // yellow
	case status >= 500:
		return "\033[38;5;196m" // red
	default:
		return "\033[38;5;245m" // gray
	}
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return path[:maxLen-3] + "..."
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
