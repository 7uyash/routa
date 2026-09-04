package recorder

import (
	"testing"
	"time"
)

func makeEntry(method, path string, status int, isReplay bool) *Entry {
	return &Entry{
		Timestamp:  time.Now(),
		Method:     method,
		Path:       path,
		StatusCode: status,
		Duration:   42 * time.Millisecond,
		IsReplay:   isReplay,
		Source:     "tunnel",
	}
}

func TestRecorderRecord(t *testing.T) {
	r := New(10)
	r.Record(makeEntry("GET", "/api/users", 200, false))
	r.Record(makeEntry("POST", "/api/users", 201, false))

	if got := r.Count(); got != 2 {
		t.Errorf("Count() = %d, want 2", got)
	}
}

func TestRecorderMaxCapacity(t *testing.T) {
	r := New(3)
	for i := 0; i < 5; i++ {
		r.Record(makeEntry("GET", "/", 200, false))
	}
	if got := r.Count(); got != 3 {
		t.Errorf("Count() = %d, want 3 (ring buffer limit)", got)
	}
}

func TestRecorderClear(t *testing.T) {
	r := New(10)
	r.Record(makeEntry("GET", "/", 200, false))
	r.Record(makeEntry("POST", "/", 201, false))
	r.Clear()
	if got := r.Count(); got != 0 {
		t.Errorf("after Clear(), Count() = %d, want 0", got)
	}
}

func TestRecorderFilterByMethod(t *testing.T) {
	r := New(50)
	r.Record(makeEntry("GET", "/a", 200, false))
	r.Record(makeEntry("POST", "/b", 201, false))
	r.Record(makeEntry("GET", "/c", 200, false))

	results := r.List(Filter{Method: "GET", Limit: 50})
	if len(results) != 2 {
		t.Errorf("filter GET: got %d, want 2", len(results))
	}
}

func TestRecorderFilterByStatus(t *testing.T) {
	r := New(50)
	r.Record(makeEntry("GET", "/ok", 200, false))
	r.Record(makeEntry("POST", "/created", 201, false))
	r.Record(makeEntry("GET", "/err", 404, false))
	r.Record(makeEntry("GET", "/srv", 500, false))

	results := r.List(Filter{StatusMin: 400, StatusMax: 599, Limit: 50})
	if len(results) != 2 {
		t.Errorf("filter 4xx-5xx: got %d, want 2", len(results))
	}
}

func TestRecorderFilterBySearch(t *testing.T) {
	r := New(50)
	r.Record(makeEntry("GET", "/api/users", 200, false))
	r.Record(makeEntry("GET", "/api/posts", 200, false))
	r.Record(makeEntry("GET", "/health", 200, false))

	results := r.List(Filter{Search: "api", Limit: 50})
	if len(results) != 2 {
		t.Errorf("filter search 'api': got %d, want 2", len(results))
	}
}

func TestRecorderFilterIsReplay(t *testing.T) {
	r := New(50)
	r.Record(makeEntry("GET", "/a", 200, false))
	r.Record(makeEntry("GET", "/b", 200, true))
	r.Record(makeEntry("GET", "/c", 200, true))

	yes := true
	no := false

	replays := r.List(Filter{IsReplay: &yes, Limit: 50})
	if len(replays) != 2 {
		t.Errorf("filter is_replay=true: got %d, want 2", len(replays))
	}

	originals := r.List(Filter{IsReplay: &no, Limit: 50})
	if len(originals) != 1 {
		t.Errorf("filter is_replay=false: got %d, want 1", len(originals))
	}
}

func TestRecorderGet(t *testing.T) {
	r := New(10)
	entry := makeEntry("GET", "/test", 200, false)
	r.Record(entry)

	got := r.Get(entry.ID)
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Path != "/test" {
		t.Errorf("Get().Path = %q, want %q", got.Path, "/test")
	}
}

func TestRecorderGetNotFound(t *testing.T) {
	r := New(10)
	got := r.Get("nonexistent-id")
	if got != nil {
		t.Errorf("Get(nonexistent) = %v, want nil", got)
	}
}

func TestRecorderListNewestFirst(t *testing.T) {
	r := New(50)
	r.Record(makeEntry("GET", "/first", 200, false))
	r.Record(makeEntry("GET", "/second", 200, false))
	r.Record(makeEntry("GET", "/third", 200, false))

	results := r.List(Filter{Limit: 50})
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Newest first
	if results[0].Path != "/third" {
		t.Errorf("first result path = %q, want /third", results[0].Path)
	}
	if results[2].Path != "/first" {
		t.Errorf("last result path = %q, want /first", results[2].Path)
	}
}

func TestRecorderOnChange(t *testing.T) {
	r := New(10)
	received := make(chan *Entry, 1)
	r.OnChange(func(e *Entry) {
		received <- e
	})

	entry := makeEntry("POST", "/notify", 201, false)
	r.Record(entry)

	select {
	case got := <-received:
		if got.Path != "/notify" {
			t.Errorf("OnChange entry path = %q, want /notify", got.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("OnChange callback was not called")
	}
}
