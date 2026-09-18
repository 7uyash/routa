// Package proxy forwards HTTP requests to the developer's local service.
package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/7uyash/routa/traffic"
)

// DefaultTransport is the shared, production-tuned http.Transport for Routa's
// local developer gateway. It is configured to handle high concurrency bursts
// to local backends without connection churn or socket exhaustion:
//   - MaxIdleConns (200): Total idle connection pool size across all hosts.
//   - MaxIdleConnsPerHost (100): Overrides Go's default of 2 to eliminate socket
//     churn (TIME_WAIT spikes) when proxying thousands of requests to a single local target.
//   - IdleConnTimeout (90s): Closes idle connections after 90s, matching Go stdlib defaults.
//   - DialContext (Timeout: 10s, KeepAlive: 30s): Quick connection establishment for local/internal
//     endpoints while enabling TCP keepalive probes.
//   - TLSHandshakeTimeout (10s): Prevents hung TLS handshakes for local HTTPS services.
//   - ResponseHeaderTimeout (60s): Bounds the time waiting for server response headers while
//     leaving ample headroom for slow local endpoints, breakpoints, or long computations.
//   - ExpectContinueTimeout (1s): Standard wait time before transmitting large request bodies.
//   - ForceAttemptHTTP2 (true): Enables HTTP/2 support when supported by the upstream target.
var DefaultTransport http.RoundTripper = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          200,
	MaxIdleConnsPerHost:   100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 60 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// Forwarder sends HTTP requests to a local target and captures timing.
type Forwarder struct {
	client *http.Client
}

// New creates a Forwarder with sensible timeout defaults and the shared gateway transport.
func New() *Forwarder {
	return NewWithTransport(DefaultTransport)
}

// NewWithTransport creates a Forwarder using the provided RoundTripper.
func NewWithTransport(transport http.RoundTripper) *Forwarder {
	return &Forwarder{
		client: &http.Client{
			Transport: transport,
			Timeout:   120 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects — return them to the caller as-is.
				return http.ErrUseLastResponse
			},
		},
	}
}

// Transport returns the RoundTripper transport configured on the Forwarder's HTTP client.
func (f *Forwarder) Transport() http.RoundTripper {
	return f.client.Transport
}

// Forward sends the request to the given target URL and returns the response
// with a timing breakdown. It does not follow redirects.
func (f *Forwarder) Forward(req traffic.Request, targetURL string) (*traffic.Response, error) {
	start := time.Now()

	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = strings.NewReader(string(req.Body))
	}

	httpReq, err := http.NewRequest(req.Method, targetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Copy headers, skipping hop-by-hop headers that shouldn't be forwarded.
	for k, vals := range req.Headers {
		key := strings.ToLower(k)
		if key == "host" || key == "connection" || key == "upgrade" ||
			key == "transfer-encoding" || key == "keep-alive" ||
			key == "proxy-connection" || key == "te" || key == "trailer" {
			continue
		}
		for _, v := range vals {
			httpReq.Header.Add(k, v)
		}
	}

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("forward request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	total := time.Since(start)

	// Collect response headers.
	respHeaders := make(http.Header)
	for k, v := range resp.Header {
		respHeaders[k] = v
	}

	return &traffic.Response{
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       respBody,
		Timing: &traffic.Timing{
			Total: total,
		},
	}, nil
}
