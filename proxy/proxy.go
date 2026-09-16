// Package proxy forwards HTTP requests to the developer's local service.
package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/7uyash/routa/traffic"
)

// Forwarder sends HTTP requests to a local target and captures timing.
type Forwarder struct {
	client *http.Client
}

// New creates a Forwarder with sensible timeout defaults.
func New() *Forwarder {
	return &Forwarder{
		client: &http.Client{
			Timeout: 120 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects — return them to the caller as-is.
				return http.ErrUseLastResponse
			},
		},
	}
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
