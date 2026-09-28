package shadow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/7uyash/routa/config"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/traffic"
)

func shadowTargets(t *testing.T) (*Shadower, []string) {
	t.Helper()

	var targets []string

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"shadow":"ok"}`))
	}))
	t.Cleanup(ok.Close)
	targets = append(targets, ok.URL)

	boom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(boom.Close)
	targets = append(targets, boom.URL)

	return New(config.ShadowConfig{Enabled: true, Targets: targets}), targets
}

func newEntry(rec *recorder.Recorder) *recorder.Entry {
	entry := &recorder.Entry{
		Method:         http.MethodGet,
		Path:           "/shadowed",
		Host:           "localhost",
		Source:         "local_proxy",
		RequestHeaders: map[string][]string{"Accept": {"application/json"}},
	}
	rec.Record(entry)
	return entry
}

func TestShadowStoresResults(t *testing.T) {
	shadower, targets := shadowTargets(t)
	rec := recorder.New(10)
	entry := newEntry(rec)

	if shadower.TargetCount() != 2 {
		t.Fatalf("TargetCount() = %d, want 2", shadower.TargetCount())
	}

	shadower.Shadow(entry, traffic.Request{Method: http.MethodGet, Path: "/shadowed", Host: "localhost"})

	results := entry.GetShadowResults()
	if len(results) != len(targets) {
		t.Fatalf("got %d shadow results, want %d", len(results), len(targets))
	}

	byTarget := make(map[string]recorder.ShadowResult, len(results))
	for _, r := range results {
		byTarget[r.Target] = r
	}

	got, ok := byTarget[targets[0]]
	if !ok {
		t.Fatalf("no result for %s, got %v", targets[0], byTarget)
	}
	if got.StatusCode != http.StatusAccepted {
		t.Errorf("shadow status = %d, want %d", got.StatusCode, http.StatusAccepted)
	}
	if string(got.ResponseBody) != `{"shadow":"ok"}` {
		t.Errorf("shadow body = %q, want %q", got.ResponseBody, `{"shadow":"ok"}`)
	}
	if got.ResponseHeaders["Content-Type"][0] != "application/json" {
		t.Errorf("shadow content-type = %q, want application/json", got.ResponseHeaders["Content-Type"][0])
	}

	if got := byTarget[targets[1]].StatusCode; got != http.StatusInternalServerError {
		t.Errorf("shadow status = %d, want %d", got, http.StatusInternalServerError)
	}
}

func TestShadowResultsReachTheRecorder(t *testing.T) {
	shadower, _ := shadowTargets(t)
	rec := recorder.New(10)
	entry := newEntry(rec)

	// This is the ordering the agent uses: shadow is dispatched before the
	// entry is recorded, and its results land afterwards.
	go shadower.Shadow(entry, traffic.Request{Method: http.MethodGet, Path: "/shadowed"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := len(rec.Get(entry.ID).GetShadowResults()); got == 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("shadow results never showed up through Recorder.Get")
}

func TestShadowResultsDoNotAliasResponseBuffers(t *testing.T) {
	shadower, _ := shadowTargets(t)
	rec := recorder.New(10)
	entry := newEntry(rec)

	shadower.Shadow(entry, traffic.Request{Method: http.MethodGet, Path: "/shadowed"})

	// A reader rewriting a result must not corrupt entry state, because the
	// dashboard can hold on to a returned slice.
	scratch := entry.GetShadowResults()
	if len(scratch) == 0 {
		t.Fatal("no shadow results to inspect")
	}
	scratch[0].StatusCode = 999
	scratch[0].ResponseBody[0] = 'X'

	fresh := entry.GetShadowResults()
	if fresh[0].StatusCode == 999 {
		t.Error("GetShadowResults() returned the internal slice")
	}
	if fresh[0].ResponseBody[0] != '{' {
		t.Errorf("shadow body = %q, want the untouched body", fresh[0].ResponseBody)
	}
}

// TestShadowUpdatesDuringDashboardReads exercises the overlap that used to race
// in production: shadow traffic writes results while the dashboard encodes
// entries. Run with -race to get the full value of it.
func TestShadowUpdatesDuringDashboardReads(t *testing.T) {
	shadower, _ := shadowTargets(t)
	rec := recorder.New(10)
	entry := newEntry(rec)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Repeated shadow rounds, like traffic arriving over a session.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				shadower.Shadow(entry, traffic.Request{Method: http.MethodGet, Path: "/shadowed"})
			}
		}
	}()

	for i := 0; i < 50; i++ {
		// handleRequestDetail encodes a snapshot from the recorder.
		if _, err := json.Marshal(rec.Get(entry.ID)); err != nil {
			t.Fatalf("json.Marshal(Get()) error = %v", err)
		}
		// handleSessions / handleAPIMap read every entry.
		for _, e := range rec.All() {
			if _, err := json.Marshal(e); err != nil {
				t.Fatalf("json.Marshal(All()) error = %v", err)
			}
		}
		// broadcastEntry reads a summary off the live entry.
		_ = entry.Summary()
	}

	close(stop)
	wg.Wait()

	if got := len(rec.Get(entry.ID).GetShadowResults()); got != 2 {
		t.Errorf("shadow results = %d, want 2", got)
	}
}

func TestShadowWithoutTargetsIsNoop(t *testing.T) {
	shadower := New(config.ShadowConfig{})
	entry := &recorder.Entry{Method: http.MethodGet, Path: "/nothing"}

	shadower.Shadow(entry, traffic.Request{Method: http.MethodGet, Path: "/nothing"})

	if shadower.TargetCount() != 0 {
		t.Errorf("TargetCount() = %d, want 0", shadower.TargetCount())
	}
	if got := entry.GetShadowResults(); got != nil {
		t.Errorf("GetShadowResults() = %v, want nil", got)
	}
}
