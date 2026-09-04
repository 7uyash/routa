// Package recorder captures HTTP request/response pairs for the inspector dashboard.
package recorder

import (
	"sync"
	"time"

	"routa/protocol"
)

// Entry represents a single captured HTTP request/response exchange.
type Entry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Request fields
	Method         string              `json:"method"`
	Path           string              `json:"path"`
	Query          string              `json:"query"`
	RequestHeaders map[string][]string `json:"request_headers"`
	RequestBody    []byte              `json:"request_body"`
	Host           string              `json:"host"`
	FullURL        string              `json:"full_url"`

	// Response fields
	StatusCode      int                 `json:"status_code"`
	ResponseHeaders map[string][]string `json:"response_headers"`
	ResponseBody    []byte              `json:"response_body"`

	// Timing
	Duration        time.Duration        `json:"duration_ms"`
	TimingBreakdown *protocol.TimingInfo `json:"timing_breakdown,omitempty"`

	// Metadata
	IsReplay   bool     `json:"is_replay"`
	OriginalID string   `json:"original_id,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Source     string   `json:"source"` // "tunnel", "replay", "webhook"
	WebhookID  string   `json:"webhook_id,omitempty"`
	Error      string   `json:"error,omitempty"`

	// Shadow traffic and diffing
	mu            sync.RWMutex
	ShadowResults []ShadowResult `json:"shadow_results,omitempty"`
	Diff          *DiffResult    `json:"diff,omitempty"`
}

// ShadowResult stores the outcome of sending a request to a shadow target.
type ShadowResult struct {
	Target          string              `json:"target"`
	StatusCode      int                 `json:"status_code"`
	ResponseHeaders map[string][]string `json:"response_headers"`
	ResponseBody    []byte              `json:"response_body"`
	Duration        time.Duration       `json:"duration"`
	Error           string              `json:"error,omitempty"`
}

// DiffResult holds the comparison between this request and its original.
type DiffResult struct {
	StatusDiff  string            `json:"status_diff,omitempty"`  // e.g. "200 -> 500"
	HeadersDiff map[string]string `json:"headers_diff,omitempty"` // added/removed/changed
	BodyDiff    string            `json:"body_diff,omitempty"`    // text diff
}

// SetShadowResults safely sets the shadow results on an entry.
func (e *Entry) SetShadowResults(results []ShadowResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ShadowResults = results
}

// GetShadowResults safely retrieves the shadow results.
func (e *Entry) GetShadowResults() []ShadowResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ShadowResults
}

// EntrySummary is a compact representation for list views.
type EntrySummary struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	Duration   int64     `json:"duration_ms"`
	IsReplay   bool      `json:"is_replay"`
	Source     string    `json:"source"`
}

// Summary returns a compact summary of the entry.
func (e *Entry) Summary() EntrySummary {
	return EntrySummary{
		ID:         e.ID,
		Timestamp:  e.Timestamp,
		Method:     e.Method,
		Path:       e.Path,
		StatusCode: e.StatusCode,
		Duration:   e.Duration.Milliseconds(),
		IsReplay:   e.IsReplay,
		Source:     e.Source,
	}
}

// Filter defines criteria for querying recorded entries.
type Filter struct {
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	StatusMin  int    `json:"status_min,omitempty"`
	StatusMax  int    `json:"status_max,omitempty"`
	Search     string `json:"search,omitempty"`
	Source     string `json:"source,omitempty"`
	IsReplay   *bool  `json:"is_replay,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}
