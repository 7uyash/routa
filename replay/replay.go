// Package replay re-sends captured requests to the local service.
package replay

import (
	"github.com/7uyash/routa/proxy"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/traffic"
)

// Engine replays recorded requests against a local service.
type Engine struct {
	executor *Executor
	rec      *recorder.Recorder
}

// New creates a replay Engine.
func New(forwarder *proxy.Forwarder, rec *recorder.Recorder) *Engine {
	return &Engine{
		executor: NewExecutor(forwarder, rec),
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

	targetURL := BuildTargetURL(localTarget, original.Path, original.Query)

	req := traffic.Request{
		Method:  original.Method,
		Path:    original.Path,
		Query:   original.Query,
		Headers: original.RequestHeaders,
		Body:    original.RequestBody,
		Host:    original.Host,
	}

	opts := ExecuteOptions{
		Source:     "replay",
		IsReplay:   true,
		OriginalID: original.ID,
		Tags:       []string{"replay"},
		Host:       original.Host,
		Record:     true,
	}

	entry, _, err := e.executor.Execute(req, targetURL, opts)
	return entry, err
}

// EditAndReplay sends a modified request to the local service.
func (e *Engine) EditAndReplay(req EditRequest, localTarget string) (*recorder.Entry, error) {
	targetURL := BuildTargetURL(localTarget, req.Path, req.Query)

	tfReq := traffic.Request{
		Method:  req.Method,
		Path:    req.Path,
		Query:   req.Query,
		Headers: req.Headers,
		Body:    req.Body,
	}

	opts := ExecuteOptions{
		Source:     "replay",
		IsReplay:   true,
		OriginalID: req.OriginalID,
		Tags:       []string{"replay", "edited"},
		Record:     true,
	}

	entry, _, err := e.executor.Execute(tfReq, targetURL, opts)
	return entry, err
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
