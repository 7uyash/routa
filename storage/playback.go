package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/7uyash/routa/proxy"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/replay"
	"github.com/7uyash/routa/traffic"
)

// PlaybackEngine plays a saved session deterministically.
type PlaybackEngine struct {
	store    *Store
	executor *replay.Executor
}

// NewPlayback creates a new playback engine.
func NewPlayback(store *Store, p *proxy.Forwarder, r *recorder.Recorder) *PlaybackEngine {
	return &PlaybackEngine{
		store:    store,
		executor: replay.NewExecutor(p, r),
	}
}

// PlaybackOptions configures how a session is played back.
type PlaybackOptions struct {
	SessionName   string `json:"session_name"`
	Target        string `json:"target"`
	MaintainDelay bool   `json:"maintain_delay"`
}

// Play runs the session and records the results.
func (p *PlaybackEngine) Play(ctx context.Context, opts PlaybackOptions) error {
	session, err := p.store.LoadSession(opts.SessionName)
	if err != nil {
		return err
	}

	if len(session.Entries) == 0 {
		return fmt.Errorf("session is empty")
	}

	// Session entries are recorded in newest-first or oldest-first?
	// The recorder.All() currently returns them in an order. Let's make sure we iterate
	// in chronological order (oldest first).
	// In recorder, usually newest is at front if we use List, but All() depends on implementation.
	// For playback, we'll sort them by timestamp.
	entries := session.Entries

	// Sort chronologically
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Timestamp.After(entries[j].Timestamp) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	var lastTimestamp time.Time

	for _, orig := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if opts.MaintainDelay && !lastTimestamp.IsZero() {
			delay := orig.Timestamp.Sub(lastTimestamp)
			if delay > 0 && delay < 30*time.Second {
				time.Sleep(delay)
			}
		}
		lastTimestamp = orig.Timestamp

		fullURL := replay.BuildTargetURL(opts.Target, orig.Path, orig.Query)

		req := traffic.Request{
			Method:  orig.Method,
			Path:    orig.Path,
			Query:   orig.Query,
			Headers: orig.RequestHeaders,
			Body:    orig.RequestBody,
			Host:    orig.Host,
		}

		execOpts := replay.ExecuteOptions{
			Source:     "playback",
			IsReplay:   true,
			OriginalID: orig.ID,
			Host:       orig.Host,
			Record:     true,
		}

		p.executor.Execute(req, fullURL, execOpts)
	}

	return nil
}
