package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/7uyash/routa/traffic"
)

func TestForwarderUsesConfiguredTransport(t *testing.T) {
	// 1. Verify New() uses DefaultTransport.
	f := New()
	if f.Transport() != DefaultTransport {
		t.Errorf("expected forwarder to use DefaultTransport, got %v", f.Transport())
	}

	transport, ok := DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatalf("DefaultTransport is not *http.Transport, got %T", DefaultTransport)
	}

	// Verify all documented configuration values on DefaultTransport
	if transport.MaxIdleConns != 200 {
		t.Errorf("MaxIdleConns = %d, want 200", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 100 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 100", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want 90s", transport.IdleConnTimeout)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 10s", transport.TLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout != 60*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 60s", transport.ResponseHeaderTimeout)
	}
	if transport.ExpectContinueTimeout != 1*time.Second {
		t.Errorf("ExpectContinueTimeout = %v, want 1s", transport.ExpectContinueTimeout)
	}
	if !transport.ForceAttemptHTTP2 {
		t.Errorf("ForceAttemptHTTP2 = false, want true")
	}

	// 2. Verify NewWithTransport uses the custom transport.
	customTransport := &http.Transport{
		MaxIdleConns: 42,
	}
	fCustom := NewWithTransport(customTransport)
	if fCustom.Transport() != customTransport {
		t.Errorf("expected custom transport, got %v", fCustom.Transport())
	}
}

func TestForwarderPreservesHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("X-Backend-Response", "from-target")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	f := New()

	req := traffic.Request{
		Method: "POST",
		Path:   "/api/data",
		Headers: http.Header{
			"X-Custom-Req":      []string{"my-value"},
			"Authorization":     []string{"Bearer token-xyz"},
			"Connection":        []string{"keep-alive"},
			"Upgrade":           []string{"websocket"},
			"Keep-Alive":        []string{"timeout=5"},
			"Transfer-Encoding": []string{"chunked"},
			"Te":                []string{"trailers"},
			"Trailer":           []string{"X-Trailer"},
			"Proxy-Connection":  []string{"close"},
		},
		Body: []byte(`{"payload":"test"}`),
	}

	resp, err := f.Forward(req, server.URL)
	if err != nil {
		t.Fatalf("Forward failed: %v", err)
	}

	// Verify standard and custom request headers were preserved
	if got := receivedHeaders.Get("X-Custom-Req"); got != "my-value" {
		t.Errorf("expected X-Custom-Req: my-value, got %q", got)
	}
	if got := receivedHeaders.Get("Authorization"); got != "Bearer token-xyz" {
		t.Errorf("expected Authorization: Bearer token-xyz, got %q", got)
	}

	// Verify hop-by-hop headers were filtered out
	hopByHop := []string{
		"Upgrade",
		"Keep-Alive",
		"Proxy-Connection",
		"Trailer",
	}
	for _, h := range hopByHop {
		if val := receivedHeaders.Get(h); val != "" {
			t.Errorf("hop-by-hop header %q should not be forwarded, got %q", h, val)
		}
	}

	// Verify response headers from the target are captured
	if got := resp.Headers.Get("X-Backend-Response"); got != "from-target" {
		t.Errorf("expected response header X-Backend-Response: from-target, got %q", got)
	}
	if got := resp.Headers.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected response header Content-Type: application/json, got %q", got)
	}
	if string(resp.Body) != `{"status":"ok"}` {
		t.Errorf("unexpected response body: %q", string(resp.Body))
	}
}

func TestForwarderDoesNotFollowRedirects(t *testing.T) {
	redirectTarget := "/target-destination"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, redirectTarget, http.StatusFound) // 302 Found
			return
		}
		if r.URL.Path == redirectTarget {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("arrived at redirect target"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	f := New()

	req := traffic.Request{
		Method:  "GET",
		Path:    "/redirect",
		Headers: http.Header{},
	}

	resp, err := f.Forward(req, server.URL+"/redirect")
	if err != nil {
		t.Fatalf("Forward returned error: %v", err)
	}

	// Verify redirect is NOT followed; the 302 itself is returned to caller
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if loc := resp.Headers.Get("Location"); loc != redirectTarget {
		t.Errorf("expected Location header %q, got %q", redirectTarget, loc)
	}
}

func TestForwarderConnectionReuse(t *testing.T) {
	// Track connections to ensure keep-alive connection pooling works
	remoteAddrs := make(map[string]int)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteAddrs[r.RemoteAddr]++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fmt.Sprintf("req-%d", len(remoteAddrs)))
	}))
	defer server.Close()

	f := New()

	req := traffic.Request{
		Method:  "GET",
		Path:    "/",
		Headers: http.Header{},
	}

	// Execute sequential requests
	for i := 0; i < 5; i++ {
		resp, err := f.Forward(req, server.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d got status %d", i, resp.StatusCode)
		}
	}

	// With connection reuse (keep-alive), all requests should share the same client connection
	if len(remoteAddrs) != 1 {
		t.Errorf("expected 1 connection to be reused across sequential requests, but got %d distinct connections: %v", len(remoteAddrs), remoteAddrs)
	}
}
