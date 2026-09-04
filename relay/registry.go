// Package relay implements the public-facing edge server that accepts
// tunnel connections and routes HTTPS requests to the correct tunnel.
package relay

import (
	"sync"
)

// TunnelConn represents a connected tunnel agent.
type TunnelConn struct {
	Subdomain string
	AuthToken string
	SendFrame func([]byte) error // write binary to the WS
	ReqChans  sync.Map           // requestID → chan []byte (for response)
}

// TunnelRegistry is a thread-safe registry of active tunnel connections,
// mapping subdomains to their WebSocket connections.
type TunnelRegistry struct {
	mu      sync.RWMutex
	tunnels map[string]*TunnelConn
}

// NewRegistry creates an empty TunnelRegistry.
func NewRegistry() *TunnelRegistry {
	return &TunnelRegistry{
		tunnels: make(map[string]*TunnelConn),
	}
}

// Register adds a tunnel connection for the given subdomain.
func (r *TunnelRegistry) Register(subdomain string, conn *TunnelConn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tunnels[subdomain] = conn
}

// Unregister removes the tunnel connection for the given subdomain.
func (r *TunnelRegistry) Unregister(subdomain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tunnels, subdomain)
}

// Lookup finds the tunnel connection for the given subdomain.
func (r *TunnelRegistry) Lookup(subdomain string) *TunnelConn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tunnels[subdomain]
}

// Count returns the number of active tunnels.
func (r *TunnelRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tunnels)
}

// List returns all active subdomain names.
func (r *TunnelRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.tunnels))
	for k := range r.tunnels {
		result = append(result, k)
	}
	return result
}
