package replay_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/7uyash/routa/proxy"
	"github.com/7uyash/routa/recorder"
	"github.com/7uyash/routa/replay"
	"github.com/7uyash/routa/traffic"
)

func TestExecutorExecute(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Response", "true")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	rec := recorder.New(10)
	fwd := proxy.New()
	executor := replay.NewExecutor(fwd, rec)

	req := traffic.Request{
		Method:  "POST",
		Path:    "/api/test",
		Query:   "foo=bar",
		Headers: map[string][]string{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"hello":"world"}`),
	}

	targetURL := replay.BuildTargetURL(ts.URL, req.Path, req.Query)

	opts := replay.ExecuteOptions{
		Source:   "test_source",
		IsReplay: true,
		Tags:     []string{"test"},
		Record:   true,
	}

	entry, resp, err := executor.Execute(req, targetURL, opts)
	if err != nil {
		t.Fatalf("unexpected error executing request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if entry.StatusCode != http.StatusOK {
		t.Errorf("expected entry status 200, got %d", entry.StatusCode)
	}

	if entry.Source != "test_source" {
		t.Errorf("expected entry source 'test_source', got %q", entry.Source)
	}

	if !entry.IsReplay {
		t.Errorf("expected entry.IsReplay to be true")
	}

	if len(rec.All()) != 1 {
		t.Errorf("expected 1 recorded entry in recorder, got %d", len(rec.All()))
	}
}
