// Package recorder captures HTTP request/response pairs for the inspector dashboard.
package recorder

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/7uyash/routa/traffic"
)

// Entry represents a single captured HTTP request/response exchange.
//
// # Ownership
//
// An entry has two phases with different rules:
//
//   - Before it is recorded, the entry belongs to a single goroutine and its
//     plain fields may be read and written directly, without locking.
//   - After Recorder.Record, the recorder has taken ownership of the pointer.
//     The recorder, the dashboard and background workers such as
//     shadow.Shadower may hold it at the same time, so the plain fields are
//     frozen from that point on and ShadowResults/Diff may only be reached
//     through the accessors below.
//
// Readers that need to keep an entry around, mutate it, or marshal it while
// the original is still being updated should take a Snapshot, or use
// Recorder.Get and Recorder.All, which already return snapshots.
//
// An Entry contains a mutex, so it must never be copied by value. Work with
// *Entry, or with a *Entry produced by Snapshot.
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
	Duration        time.Duration   `json:"duration_ms"`
	TimingBreakdown *traffic.Timing `json:"timing_breakdown,omitempty"`

	// Metadata
	IsReplay   bool     `json:"is_replay"`
	OriginalID string   `json:"original_id,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Source     string   `json:"source"` // "tunnel", "replay", "webhook"
	WebhookID  string   `json:"webhook_id,omitempty"`
	Error      string   `json:"error,omitempty"`

	// Shadow traffic and diffing.
	//
	// mu guards ShadowResults and Diff. These two fields are the only entry
	// state that may change after the entry has been recorded, because they are
	// written asynchronously by shadow traffic and by the diff pipeline. Go
	// through SetShadowResults/GetShadowResults and SetDiff/GetDiff so that the
	// access is synchronised and no internal slice or map escapes.
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

// SetShadowResults replaces the entry's shadow results.
//
// The results are deep-copied, so the caller may reuse or modify its slice
// afterwards without corrupting entry state. This is safe to call after the
// entry has been recorded, which is how shadow.Shadower reports results for
// traffic that is already visible in the dashboard.
func (e *Entry) SetShadowResults(results []ShadowResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ShadowResults = cloneShadowResults(results)
}

// GetShadowResults returns a deep copy of the entry's shadow results.
//
// Returning a copy keeps the caller from reaching back into entry state once
// the lock has been released, and from racing with a concurrent
// SetShadowResults. It returns nil when no shadow traffic was recorded.
func (e *Entry) GetShadowResults() []ShadowResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return cloneShadowResults(e.ShadowResults)
}

// SetDiff stores a deep copy of a diff result on the entry, replacing any
// previous one. A nil result clears the diff.
func (e *Entry) SetDiff(d *DiffResult) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Diff = cloneDiff(d)
}

// GetDiff returns a deep copy of the entry's diff result, or nil if the entry
// has not been diffed.
func (e *Entry) GetDiff() *DiffResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return cloneDiff(e.Diff)
}

// Snapshot returns a deep copy of the entry that shares no memory with the
// receiver.
//
// The copy is an ordinary, freely mutable value: callers may edit it, keep it
// around after the original has been evicted from the recorder, and marshal it
// while shadow results keep arriving. It is the ownership boundary for readers,
// and is what Recorder.Get and Recorder.All hand out.
func (e *Entry) Snapshot() *Entry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &Entry{
		ID:        e.ID,
		Timestamp: e.Timestamp,

		Method:         e.Method,
		Path:           e.Path,
		Query:          e.Query,
		RequestHeaders: cloneHeaders(e.RequestHeaders),
		RequestBody:    cloneBytes(e.RequestBody),
		Host:           e.Host,
		FullURL:        e.FullURL,

		StatusCode:      e.StatusCode,
		ResponseHeaders: cloneHeaders(e.ResponseHeaders),
		ResponseBody:    cloneBytes(e.ResponseBody),

		Duration:        e.Duration,
		TimingBreakdown: e.TimingBreakdown,

		IsReplay:   e.IsReplay,
		OriginalID: e.OriginalID,
		Tags:       cloneStrings(e.Tags),
		Source:     e.Source,
		WebhookID:  e.WebhookID,
		Error:      e.Error,

		ShadowResults: cloneShadowResults(e.ShadowResults),
		Diff:          cloneDiff(e.Diff),
	}
}

// MarshalJSON encodes the entry while holding the read lock, so a concurrent
// SetShadowResults or SetDiff cannot race with a dashboard read.
//
// entryAlias has the same fields as Entry but none of its methods, which is
// what stops encoding/json from recursing back into this function. The
// conversion is a pointer conversion, so the mutex is never copied.
func (e *Entry) MarshalJSON() ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	type entryAlias Entry
	return json.Marshal((*entryAlias)(e))
}

// cloneBytes returns a copy of b, preserving nil.
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// cloneStrings returns a copy of s, preserving nil.
func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

// cloneHeaders deep-copies a multi-value header map, preserving nil.
func cloneHeaders(h map[string][]string) map[string][]string {
	if h == nil {
		return nil
	}
	out := make(map[string][]string, len(h))
	for k, v := range h {
		out[k] = cloneStrings(v)
	}
	return out
}

// cloneShadowResults deep-copies shadow results, including each result's
// headers and body, so no internal map or slice is shared with the caller.
func cloneShadowResults(results []ShadowResult) []ShadowResult {
	if results == nil {
		return nil
	}
	out := make([]ShadowResult, len(results))
	for i, r := range results {
		r.ResponseHeaders = cloneHeaders(r.ResponseHeaders)
		r.ResponseBody = cloneBytes(r.ResponseBody)
		out[i] = r
	}
	return out
}

// cloneDiff deep-copies a diff result, preserving nil.
func cloneDiff(d *DiffResult) *DiffResult {
	if d == nil {
		return nil
	}
	out := *d
	if d.HeadersDiff != nil {
		out.HeadersDiff = make(map[string]string, len(d.HeadersDiff))
		for k, v := range d.HeadersDiff {
			out.HeadersDiff[k] = v
		}
	}
	return &out
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
