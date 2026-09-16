package replay

import (
	"strings"
	"time"

	"github.com/7uyash/routa/proxy"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/traffic"
)

// ExecuteOptions configures execution parameters for Executor.Execute.
type ExecuteOptions struct {
	Source     string   // e.g., "replay", "playback", "scenario"
	IsReplay   bool     // set to true for replayed/played back entries
	OriginalID string   // optional original entry ID
	Tags       []string // optional tags for entry recording
	Host       string   // optional Host override for Entry
	Record     bool     // whether to record entry into recorder (default true)
}

// Executor provides a centralized execution path for forwarding traffic.Request,
// tracking execution timing, handling forwarding errors, constructing recorder.Entry,
// and optionally saving entries into recorder.Recorder.
type Executor struct {
	Forwarder *proxy.Forwarder
	Recorder  *recorder.Recorder
}

// NewExecutor creates a new Executor.
func NewExecutor(forwarder *proxy.Forwarder, rec *recorder.Recorder) *Executor {
	return &Executor{
		Forwarder: forwarder,
		Recorder:  rec,
	}
}

// BuildTargetURL constructs a full target URL from target base and path/query.
func BuildTargetURL(target, path, query string) string {
	target = strings.TrimSuffix(target, "/")
	fullURL := target + path
	if query != "" {
		fullURL += "?" + query
	}
	return fullURL
}

// Execute forwards a traffic.Request to targetURL, measures timing, constructs a recorder.Entry,
// records it if requested, and returns the constructed entry along with any forwarder error.
func (e *Executor) Execute(req traffic.Request, targetURL string, opts ExecuteOptions) (*recorder.Entry, *traffic.Response, error) {
	start := time.Now()

	host := req.Host
	if opts.Host != "" {
		host = opts.Host
	}

	resp, err := e.Forwarder.Forward(req, targetURL)

	entry := &recorder.Entry{
		Timestamp:      time.Now(),
		Method:         req.Method,
		Path:           req.Path,
		Query:          req.Query,
		RequestHeaders: req.Headers,
		RequestBody:    req.Body,
		Host:           host,
		FullURL:        targetURL,
		IsReplay:       opts.IsReplay,
		OriginalID:     opts.OriginalID,
		Source:         opts.Source,
		Tags:           opts.Tags,
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

	if e.Recorder != nil && (opts.Record || opts.Source != "") {
		e.Recorder.Record(entry)
	}

	return entry, resp, err
}
