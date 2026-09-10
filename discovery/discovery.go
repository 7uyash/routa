// Package discovery provides zero-config local service discovery,
// port scanning, friendly service name inference, and safe route proposals.
package discovery

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CommonPorts to scan for local HTTP services.
var CommonPorts = []int{
	3000, 3001, 3002, 4000, 4200, 5000, 5001, 5173, 8000, 8001,
	8080, 8081, 8082, 8088, 8888, 9000, 9090, 9091,
}

// Service represents a discovered local HTTP service.
type Service struct {
	ID           string    `json:"id"`
	Port         int       `json:"port"`
	URL          string    `json:"url"`
	Status       string    `json:"status"` // "active", "unresponsive"
	ServerHeader string    `json:"server_header,omitempty"`
	FriendlyName string    `json:"friendly_name"`
	IsRouted     bool      `json:"is_routed"`
	LastSeen     time.Time `json:"last_seen"`
}

// RouteProposal represents a proposed routing rule requiring user confirmation.
type RouteProposal struct {
	ID             string    `json:"id"`
	PathPattern    string    `json:"path_pattern"`
	TargetURL      string    `json:"target_url"`
	ServiceName    string    `json:"service_name"`
	Status         string    `json:"status"` // "proposed", "confirmed", "rejected"
	CreatedAt      time.Time `json:"created_at"`
}

// Scanner manages local port scanning and route proposals.
type Scanner struct {
	mu          sync.RWMutex
	services    map[int]*Service
	proposals   map[string]*RouteProposal
	activePorts map[int]bool // ports currently configured in routes
}

// NewScanner creates a new discovery Scanner.
func NewScanner() *Scanner {
	return &Scanner{
		services:    make(map[int]*Service),
		proposals:   make(map[string]*RouteProposal),
		activePorts: make(map[int]bool),
	}
}

// SetActivePorts updates which ports are currently routed.
func (s *Scanner) SetActivePorts(ports []int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.activePorts = make(map[int]bool)
	for _, p := range ports {
		s.activePorts[p] = true
	}

	for p, svc := range s.services {
		svc.IsRouted = s.activePorts[p]
	}
}

// Scan performs a fast concurrent scan of common ports.
func (s *Scanner) Scan() []*Service {
	var wg sync.WaitGroup
	client := &http.Client{
		Timeout: 300 * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	results := make(chan *Service, len(CommonPorts))

	for _, port := range CommonPorts {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			svc := probePort(client, p)
			if svc != nil {
				results <- svc
			}
		}(port)
	}

	wg.Wait()
	close(results)

	s.mu.Lock()
	defer s.mu.Unlock()

	discovered := make([]*Service, 0)
	for svc := range results {
		svc.IsRouted = s.activePorts[svc.Port]
		s.services[svc.Port] = svc
		discovered = append(discovered, svc)
	}

	return discovered
}

// GetServices returns all currently discovered services.
func (s *Scanner) GetServices() []*Service {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*Service, 0, len(s.services))
	for _, svc := range s.services {
		res = append(res, svc)
	}
	return res
}

// ProposeRoute registers a new proposed routing rule.
func (s *Scanner) ProposeRoute(pattern, targetURL, serviceName string) *RouteProposal {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("prop_%d", time.Now().UnixNano())
	prop := &RouteProposal{
		ID:          id,
		PathPattern: pattern,
		TargetURL:   targetURL,
		ServiceName: serviceName,
		Status:      "proposed",
		CreatedAt:   time.Now(),
	}

	s.proposals[id] = prop
	return prop
}

// ConfirmProposal marks a proposal as confirmed.
func (s *Scanner) ConfirmProposal(id string) (*RouteProposal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prop, ok := s.proposals[id]
	if !ok {
		return nil, false
	}

	prop.Status = "confirmed"
	return prop, true
}

// GetProposals returns all proposals.
func (s *Scanner) GetProposals() []*RouteProposal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]*RouteProposal, 0, len(s.proposals))
	for _, prop := range s.proposals {
		res = append(res, prop)
	}
	return res
}

// probePort checks if a local port is serving HTTP traffic and infers friendly tech stack name.
func probePort(client *http.Client, port int) *Service {
	targetURL := fmt.Sprintf("http://localhost:%d", port)

	// First check if TCP port is open (try localhost / dual-stack)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 200*time.Millisecond)
	if err != nil {
		// Fallback to 127.0.0.1
		conn, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err != nil {
			return nil
		}
	}
	conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		// Port open but failed GET - try HEAD
		return &Service{
			ID:           fmt.Sprintf("svc_%d", port),
			Port:         port,
			URL:          targetURL,
			Status:       "active",
			FriendlyName: inferFriendlyName(port, "", "", ""),
			LastSeen:     time.Now(),
		}
	}
	defer resp.Body.Close()

	serverHeader := resp.Header.Get("Server")
	poweredBy := resp.Header.Get("X-Powered-By")
	contentType := resp.Header.Get("Content-Type")

	friendly := inferFriendlyName(port, serverHeader, poweredBy, contentType)

	return &Service{
		ID:           fmt.Sprintf("svc_%d", port),
		Port:         port,
		URL:          targetURL,
		Status:       "active",
		ServerHeader: serverHeader,
		FriendlyName: friendly,
		LastSeen:     time.Now(),
	}
}

// inferFriendlyName guesses technology stack based on port and headers.
func inferFriendlyName(port int, server, poweredBy, contentType string) string {
	s := strings.ToLower(server + " " + poweredBy)

	if strings.Contains(s, "express") || strings.Contains(s, "node") {
		return fmt.Sprintf("Node.js / Express App (:%d)", port)
	}
	if strings.Contains(s, "next.js") || strings.Contains(s, "next") {
		return fmt.Sprintf("Next.js App (:%d)", port)
	}
	if strings.Contains(s, "gunicorn") || strings.Contains(s, "uvicorn") || strings.Contains(s, "fastapi") {
		return fmt.Sprintf("Python FastAPI / Uvicorn (:%d)", port)
	}
	if strings.Contains(s, "workman") || strings.Contains(s, "apache") || strings.Contains(s, "nginx") || strings.Contains(s, "php") {
		return fmt.Sprintf("PHP / Web Server (:%d)", port)
	}
	if strings.Contains(s, "cowboy") || strings.Contains(s, "phoenix") {
		return fmt.Sprintf("Elixir / Phoenix (:%d)", port)
	}

	// Port heuristic fallbacks
	switch port {
	case 3000:
		return "Frontend / React / Node App (:3000)"
	case 5173:
		return "Vite Dev Server (:5173)"
	case 4200:
		return "Angular Dev Server (:4200)"
	case 8000:
		return "Python / Django / FastAPI (:8000)"
	case 8080:
		return "Java Spring Boot / HTTP App (:8080)"
	case 8081:
		return "Go Microservice (:8081)"
	case 8082:
		return "Backend Microservice (:8082)"
	case 5000:
		return "Flask / Python Service (:5000)"
	case 9090:
		return "Prometheus / Shadow Target (:9090)"
	default:
		return fmt.Sprintf("HTTP Service (:%d)", port)
	}
}
