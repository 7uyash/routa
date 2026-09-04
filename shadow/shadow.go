package shadow

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/recorder"
)

// Shadower duplicates traffic to secondary targets concurrently without blocking
// the primary response.
type Shadower struct {
	targets []string
	client  *http.Client
}

// New creates a new Shadower.
func New(cfg config.ShadowConfig) *Shadower {
	var targets []string
	if cfg.Enabled {
		targets = cfg.Targets
	}
	return &Shadower{
		targets: targets,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// TargetCount returns the number of active shadow targets.
func (s *Shadower) TargetCount() int {
	return len(s.targets)
}

// Shadow takes a request intended for the primary target, duplicates it to all
// shadow targets concurrently, and updates the recorder.Entry with the results
// once they arrive. This function should be called as a goroutine.
func (s *Shadower) Shadow(entry *recorder.Entry, method, path, query string, headers map[string][]string, body []byte) {
	if len(s.targets) == 0 {
		return
	}

	var wg sync.WaitGroup
	results := make([]recorder.ShadowResult, len(s.targets))

	for i, target := range s.targets {
		wg.Add(1)
		go func(idx int, targetURL string) {
			defer wg.Done()
			start := time.Now()

			fullURL := targetURL + path
			if query != "" {
				fullURL += "?" + query
			}

			req, err := http.NewRequest(method, fullURL, bytes.NewReader(body))
			if err != nil {
				results[idx] = recorder.ShadowResult{Target: targetURL, Error: err.Error()}
				return
			}

			// Copy headers
			for k, v := range headers {
				for _, val := range v {
					req.Header.Add(k, val)
				}
			}

			resp, err := s.client.Do(req)
			if err != nil {
				results[idx] = recorder.ShadowResult{Target: targetURL, Error: err.Error()}
				return
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			duration := time.Since(start)

			results[idx] = recorder.ShadowResult{
				Target:          targetURL,
				StatusCode:      resp.StatusCode,
				ResponseHeaders: resp.Header,
				ResponseBody:    respBody,
				Duration:        duration,
			}
		}(i, target)
	}

	wg.Wait()

	// Update the entry safely (Entry must support thread-safe updates to this field if read concurrently,
	// or we just set it since Dashboard UI pulls on demand). We assume Entry is already recorded,
	// but adding ShadowResults later is fine. The UI will pick them up on refresh or diff.
	entry.SetShadowResults(results)
}
