// Package traffic provides canonical models for HTTP requests and responses
// used throughout the Routa system.
package traffic

import (
	"net/http"
	"time"
)

// Request represents a normalized HTTP request traversing Routa's pipelines.
type Request struct {
	Method  string
	Path    string
	Query   string
	Headers http.Header
	Body    []byte
	Host    string
}

// Response represents a normalized HTTP response traversing Routa's pipelines.
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Timing     *Timing
}

// Timing tracks the latency breakdown for a forwarded request.
type Timing struct {
	DNSLookup    time.Duration
	TCPConnect   time.Duration
	TLSHandshake time.Duration
	FirstByte    time.Duration
	Total        time.Duration
}
