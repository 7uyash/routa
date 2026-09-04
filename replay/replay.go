// Package replay re-sends captured requests to the local service.
package replay

import (
	"time"

	"routa/proxy"
	"routa/recorder"
)

// Engine replays recorded requests against a local service.
type Engine struct {
	forwarder *proxy.Forwarder
	rec       *recorder.Recorder
}

// New creates a replay Engine.
func New(forwarder *proxy.Forwarder, rec *recorder.Recorder) *Engine {
	return &Engine{
		forwarder: forwarder,
		rec:       rec,
	}
}

// Replay sends the exact original request to the local service and records
// the new response. The replayed entry is marked with IsReplay=true.
func (e *Engine) Replay(entryID, localTarget string) (*recorder.Entry, error) {
	original := e.rec.Get(entryID)
	if original == nil {
		return nil, ErrNotFound
	}

	targetURL := localTarget + original.Path
	if original.Query != "" {
		targetURL += "?" + original.Query
	}

	start := time.Now()
	resp, err := e.forwarder.Forward(
		original.Method,
		targetURL,
		original.RequestHeaders,
		original.RequestBody,
	)

	entry := &recorder.Entry{
		Timestamp:      time.Now(),
		Method:         original.Method,
		Path:           original.Path,
		Query:          original.Query,
		RequestHeaders: original.RequestHeaders,
		RequestBody:    original.RequestBody,
		Host:           original.Host,
		FullURL:        targetURL,
		IsReplay:       true,
		OriginalID:     original.ID,
		Source:         "replay",
		Tags:          []string{"replay"},
	}

	if err != nil {
		entry.Error = err.Error()
		entry.StatusCode = 502
		entry.Duration = time.Since(start)
	} else {
		entry.StatusCode = resp.StatusCode
		entry.ResponseHeaders = resp.Headers
		entry.ResponseBody = resp.Body
		entry.Duration = time.Since(start)
		entry.TimingBreakdown = resp.Timing
	}

	e.rec.Record(entry)
	return entry, nil
}

// EditAndReplay sends a modified request to the local service.
func (e *Engine) EditAndReplay(req EditRequest, localTarget string) (*recorder.Entry, error) {
	targetURL := localTarget + req.Path
	if req.Query != "" {
		targetURL += "?" + req.Query
	}

	start := time.Now()
	resp, err := e.forwarder.Forward(
		req.Method,
		targetURL,
		req.Headers,
		req.Body,
	)

	entry := &recorder.Entry{
		Timestamp:      time.Now(),
		Method:         req.Method,
		Path:           req.Path,
		Query:          req.Query,
		RequestHeaders: req.Headers,
		RequestBody:    req.Body,
		FullURL:        targetURL,
		IsReplay:       true,
		OriginalID:     req.OriginalID,
		Source:         "replay",
		Tags:          []string{"replay", "edited"},
	}

	if err != nil {
		entry.Error = err.Error()
		entry.StatusCode = 502
		entry.Duration = time.Since(start)
	} else {
		entry.StatusCode = resp.StatusCode
		entry.ResponseHeaders = resp.Headers
		entry.ResponseBody = resp.Body
		entry.Duration = time.Since(start)
		entry.TimingBreakdown = resp.Timing
	}

	e.rec.Record(entry)
	return entry, nil
}

// EditRequest describes a modified request for edit-and-replay.
type EditRequest struct {
	OriginalID string              `json:"original_id"`
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	Query      string              `json:"query"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
}

// ErrNotFound is returned when a requested entry doesn't exist.
type errNotFound struct{}

func (errNotFound) Error() string { return "entry not found" }

// ErrNotFound is a sentinel error for missing entries.
var ErrNotFound = errNotFound{}
