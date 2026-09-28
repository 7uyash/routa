package recorder

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

func shadowResults() []ShadowResult {
	return []ShadowResult{
		{
			Target:          "http://shadow-1",
			StatusCode:      201,
			ResponseHeaders: map[string][]string{"Content-Type": {"application/json"}},
			ResponseBody:    []byte(`{"shadow":true}`),
			Duration:        5,
		},
		{
			Target:       "http://shadow-2",
			StatusCode:   500,
			Error:        "connection refused",
			ResponseBody: []byte("boom"),
		},
	}
}

// --- shadow result accessors -------------------------------------------------

func TestGetShadowResultsReturnsCopy(t *testing.T) {
	e := makeEntry("GET", "/a", 200, false)
	e.SetShadowResults(shadowResults())

	got := e.GetShadowResults()
	if len(got) != 2 {
		t.Fatalf("GetShadowResults() len = %d, want 2", len(got))
	}

	// Mutate every mutable part of the returned slice.
	got[0].StatusCode = 999
	got[0].Target = "hijacked"
	got[0].ResponseHeaders["Content-Type"][0] = "text/html"
	got[0].ResponseBody[0] = 'X'
	got = append(got, ShadowResult{Target: "extra"})

	fresh := e.GetShadowResults()
	if fresh[0].StatusCode != 201 {
		t.Errorf("entry shadow status = %d, want 201 (caller mutated the returned slice)", fresh[0].StatusCode)
	}
	if fresh[0].Target != "http://shadow-1" {
		t.Errorf("entry shadow target = %q, want http://shadow-1", fresh[0].Target)
	}
	if got := fresh[0].ResponseHeaders["Content-Type"][0]; got != "application/json" {
		t.Errorf("entry shadow header = %q, want application/json (headers map was shared)", got)
	}
	if fresh[0].ResponseBody[0] != '{' {
		t.Errorf("entry shadow body = %q, want a JSON body (body slice was shared)", fresh[0].ResponseBody)
	}
	if len(fresh) != 2 {
		t.Errorf("entry shadow results len = %d, want 2 (caller appended to the internal slice)", len(fresh))
	}
}

func TestSetShadowResultsCopiesInput(t *testing.T) {
	e := makeEntry("GET", "/a", 200, false)

	mine := shadowResults()
	e.SetShadowResults(mine)

	// The caller keeps writing after handing the slice over.
	mine[0].StatusCode = 418
	mine[0].ResponseHeaders["Content-Type"][0] = "text/html"
	mine[0].ResponseBody[0] = 'X'
	mine[1].Target = "hijacked"

	got := e.GetShadowResults()
	if got[0].StatusCode != 201 {
		t.Errorf("entry shadow status = %d, want 201 (SetShadowResults kept the caller's slice)", got[0].StatusCode)
	}
	if h := got[0].ResponseHeaders["Content-Type"][0]; h != "application/json" {
		t.Errorf("entry shadow header = %q, want application/json", h)
	}
	if got[0].ResponseBody[0] != '{' {
		t.Errorf("entry shadow body = %q, want a JSON body", got[0].ResponseBody)
	}
	if got[1].Target != "http://shadow-2" {
		t.Errorf("entry shadow target = %q, want http://shadow-2", got[1].Target)
	}
}

func TestShadowResultsRoundTrip(t *testing.T) {
	e := makeEntry("GET", "/a", 200, false)

	if got := e.GetShadowResults(); got != nil {
		t.Errorf("GetShadowResults() on a fresh entry = %v, want nil", got)
	}

	want := shadowResults()
	e.SetShadowResults(want)

	got := e.GetShadowResults()
	if len(got) != len(want) {
		t.Fatalf("GetShadowResults() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Target != want[i].Target {
			t.Errorf("result %d target = %q, want %q", i, got[i].Target, want[i].Target)
		}
		if got[i].StatusCode != want[i].StatusCode {
			t.Errorf("result %d status = %d, want %d", i, got[i].StatusCode, want[i].StatusCode)
		}
		if string(got[i].ResponseBody) != string(want[i].ResponseBody) {
			t.Errorf("result %d body = %q, want %q", i, got[i].ResponseBody, want[i].ResponseBody)
		}
	}
}

// --- diff accessors ----------------------------------------------------------

func TestGetDiffReturnsCopy(t *testing.T) {
	e := makeEntry("GET", "/a", 200, false)
	e.SetDiff(&DiffResult{
		StatusDiff:  "200 -> 500",
		HeadersDiff: map[string]string{"Content-Type": "text/plain"},
		BodyDiff:    "-ok\n+fail",
	})

	got := e.GetDiff()
	if got == nil {
		t.Fatal("GetDiff() = nil, want a diff")
	}

	got.StatusDiff = "mutated"
	got.HeadersDiff["Content-Type"] = "mutated"

	fresh := e.GetDiff()
	if fresh.StatusDiff != "200 -> 500" {
		t.Errorf("diff status = %q, want %q (caller mutated the returned diff)", fresh.StatusDiff, "200 -> 500")
	}
	if fresh.HeadersDiff["Content-Type"] != "text/plain" {
		t.Errorf("diff header = %q, want text/plain (headers map was shared)", fresh.HeadersDiff["Content-Type"])
	}
}

func TestSetDiffCopiesInput(t *testing.T) {
	e := makeEntry("GET", "/a", 200, false)

	mine := &DiffResult{StatusDiff: "200 -> 500", HeadersDiff: map[string]string{"A": "1"}}
	e.SetDiff(mine)

	mine.StatusDiff = "mutated"
	mine.HeadersDiff["A"] = "mutated"

	got := e.GetDiff()
	if got.StatusDiff != "200 -> 500" {
		t.Errorf("diff status = %q, want %q (SetDiff kept the caller's pointer)", got.StatusDiff, "200 -> 500")
	}
	if got.HeadersDiff["A"] != "1" {
		t.Errorf("diff header = %q, want 1", got.HeadersDiff["A"])
	}
}

func TestGetDiffNilWhenUnset(t *testing.T) {
	e := makeEntry("GET", "/a", 200, false)

	if got := e.GetDiff(); got != nil {
		t.Errorf("GetDiff() on a fresh entry = %v, want nil", got)
	}

	e.SetDiff(&DiffResult{StatusDiff: "200 -> 500"})
	e.SetDiff(nil)
	if got := e.GetDiff(); got != nil {
		t.Errorf("GetDiff() after SetDiff(nil) = %v, want nil", got)
	}
}

// --- snapshot / recorder ownership ------------------------------------------

func TestSnapshotIsIndependent(t *testing.T) {
	e := makeEntry("GET", "/a", 200, false)
	e.ID = "id-1"
	e.RequestHeaders = map[string][]string{"Accept": {"application/json"}}
	e.RequestBody = []byte("request")
	e.ResponseHeaders = map[string][]string{"Content-Type": {"text/plain"}}
	e.ResponseBody = []byte("response")
	e.Tags = []string{"replay"}
	e.SetShadowResults(shadowResults())
	e.SetDiff(&DiffResult{StatusDiff: "200 -> 500", HeadersDiff: map[string]string{"A": "1"}})

	snap := e.Snapshot()
	if snap == e {
		t.Fatal("Snapshot() returned the receiver")
	}

	// Mutate every mutable part of the snapshot.
	snap.Path = "/mutated"
	snap.StatusCode = 503
	snap.RequestHeaders["Accept"][0] = "text/html"
	snap.RequestBody[0] = 'X'
	snap.ResponseHeaders["Content-Type"][0] = "application/json"
	snap.ResponseBody[0] = 'X'
	snap.Tags[0] = "mutated"
	snap.GetShadowResults()[0].ResponseBody[0] = 'X'
	snap.Diff.HeadersDiff["A"] = "mutated"

	if e.Path != "/a" || e.StatusCode != 200 {
		t.Errorf("original plain fields changed: path=%q status=%d", e.Path, e.StatusCode)
	}
	if h := e.RequestHeaders["Accept"][0]; h != "application/json" {
		t.Errorf("original request header = %q, want application/json", h)
	}
	if e.RequestBody[0] != 'r' {
		t.Errorf("original request body = %q, want %q", e.RequestBody, "request")
	}
	if h := e.ResponseHeaders["Content-Type"][0]; h != "text/plain" {
		t.Errorf("original response header = %q, want text/plain", h)
	}
	if e.ResponseBody[0] != 'r' {
		t.Errorf("original response body = %q, want %q", e.ResponseBody, "response")
	}
	if e.Tags[0] != "replay" {
		t.Errorf("original tag = %q, want replay", e.Tags[0])
	}
	if b := e.GetShadowResults()[0].ResponseBody; b[0] != '{' {
		t.Errorf("original shadow body = %q, want a JSON body", b)
	}
	if e.Diff.HeadersDiff["A"] != "1" {
		t.Errorf("original diff header = %q, want 1", e.Diff.HeadersDiff["A"])
	}
}

func TestRecorderGetReturnsSnapshot(t *testing.T) {
	r := New(10)
	entry := makeEntry("GET", "/test", 200, false)
	entry.RequestHeaders = map[string][]string{"Accept": {"application/json"}}
	r.Record(entry)

	got := r.Get(entry.ID)
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got == entry {
		t.Fatal("Get() returned the recorded entry, want a snapshot")
	}
	if got.Path != "/test" {
		t.Errorf("Get().Path = %q, want %q", got.Path, "/test")
	}

	// A caller rewriting the snapshot must not change recorded state.
	got.Path = "/mutated"
	got.StatusCode = 503
	got.RequestHeaders["Accept"][0] = "text/html"

	fresh := r.Get(entry.ID)
	if fresh.Path != "/test" {
		t.Errorf("recorded path = %q, want /test (Get() leaked the live entry)", fresh.Path)
	}
	if fresh.StatusCode != 200 {
		t.Errorf("recorded status = %d, want 200 (Get() leaked the live entry)", fresh.StatusCode)
	}
	if h := fresh.RequestHeaders["Accept"][0]; h != "application/json" {
		t.Errorf("recorded request header = %q, want application/json", h)
	}
}

func TestRecorderAllReturnsSnapshots(t *testing.T) {
	r := New(50)
	first := makeEntry("GET", "/first", 200, false)
	second := makeEntry("GET", "/second", 201, false)
	r.Record(first)
	r.Record(second)

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All() len = %d, want 2", len(all))
	}
	if all[0].Path != "/second" {
		t.Errorf("All()[0].Path = %q, want /second (newest first)", all[0].Path)
	}
	for _, e := range all {
		if e == first || e == second {
			t.Fatal("All() returned a recorded entry, want snapshots")
		}
	}

	all[0].Path = "/mutated"
	if got := r.Get(second.ID).Path; got != "/second" {
		t.Errorf("recorded path = %q, want /second (All() leaked the live entries)", got)
	}
}

func TestRecorderGetNotFoundReturnsNil(t *testing.T) {
	r := New(10)
	if got := r.Get("nope"); got != nil {
		t.Errorf("Get() = %v, want nil", got)
	}
}

// --- JSON --------------------------------------------------------------------

func TestEntryJSONKeepsShadowResultsAndDiff(t *testing.T) {
	e := makeEntry("GET", "/a", 200, false)
	e.SetShadowResults(shadowResults())
	e.SetDiff(&DiffResult{StatusDiff: "200 -> 500"})

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, ok := out["shadow_results"]; !ok {
		t.Errorf("marshalled entry is missing shadow_results: %s", data)
	}
	if _, ok := out["diff"]; !ok {
		t.Errorf("marshalled entry is missing diff: %s", data)
	}
	if _, ok := out["mu"]; ok {
		t.Errorf("marshalled entry leaked the mutex: %s", data)
	}

	results, ok := out["shadow_results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("shadow_results = %v, want 2 entries", out["shadow_results"])
	}
	first, _ := results[0].(map[string]any)
	if first["target"] != "http://shadow-1" {
		t.Errorf("shadow_results[0].target = %v, want http://shadow-1", first["target"])
	}
}

func TestEntryJSONRoundTrip(t *testing.T) {
	e := makeEntry("GET", "/api/users", 200, false)
	e.RequestBody = []byte("request")
	e.SetShadowResults(shadowResults())
	e.SetDiff(&DiffResult{
		StatusDiff:  "200 -> 500",
		HeadersDiff: map[string]string{"A": "1"},
		BodyDiff:    "-ok\n+fail",
	})

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var back Entry
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Sessions and scenarios persist entries as JSON, so shadow results and
	// diffs have to survive the round trip.
	backResults := back.GetShadowResults()
	if len(backResults) != 2 {
		t.Fatalf("decoded shadow results = %d, want 2", len(backResults))
	}
	if backResults[0].Target != "http://shadow-1" {
		t.Errorf("decoded shadow target = %q, want http://shadow-1", backResults[0].Target)
	}
	if string(backResults[0].ResponseBody) != `{"shadow":true}` {
		t.Errorf("decoded shadow body = %q, want the original body", backResults[0].ResponseBody)
	}

	backDiff := back.GetDiff()
	if backDiff == nil {
		t.Fatal("decoded diff = nil, want a diff")
	}
	if backDiff.StatusDiff != "200 -> 500" {
		t.Errorf("decoded diff status = %q, want %q", backDiff.StatusDiff, "200 -> 500")
	}
	if backDiff.HeadersDiff["A"] != "1" {
		t.Errorf("decoded diff header = %q, want 1", backDiff.HeadersDiff["A"])
	}
}

// --- concurrency -------------------------------------------------------------

// TestShadowResultsConcurrentAccess exercises the shadow/dashboard overlap that
// used to race: shadow traffic writes results while the dashboard reads them.
// Run with -race to get the full value of it.
func TestShadowResultsConcurrentAccess(t *testing.T) {
	e := makeEntry("GET", "/race", 200, false)
	r := New(10)
	r.Record(e)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			e.SetShadowResults(shadowResults())
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			for _, s := range e.GetShadowResults() {
				_ = len(s.ResponseBody)
			}
		}
	}()
	wg.Wait()

	if got := len(e.GetShadowResults()); got != 2 {
		t.Errorf("shadow results len = %d, want 2", got)
	}
}

// TestMarshalLiveEntryDuringShadowUpdates covers the paths that encode a live
// recorded entry rather than a snapshot, such as the dashboard echoing back the
// entry returned by a replay. Encoding has to take the read lock, otherwise it
// races with shadow results still landing. Run with -race to get the full value
// of it.
func TestMarshalLiveEntryDuringShadowUpdates(t *testing.T) {
	r := New(10)
	entry := makeEntry("GET", "/live", 200, false)
	r.Record(entry)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				entry.SetShadowResults(shadowResults())
			}
		}
	}()

	for i := 0; i < 500; i++ {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("json.Marshal(live entry) error = %v", err)
		}
		if !strings.Contains(string(data), `"id"`) {
			t.Fatalf("marshalled entry lost its id: %s", data)
		}
	}

	close(stop)
	wg.Wait()
}

// TestDashboardReadsDuringShadowUpdates mirrors the dashboard's real read paths
// (JSON encoding an entry, and Session/All style reads) while shadow results
// are still arriving. Run with -race to get the full value of it.
func TestDashboardReadsDuringShadowUpdates(t *testing.T) {
	r := New(10)
	entry := makeEntry("GET", "/dashboard", 200, false)
	entry.RequestHeaders = map[string][]string{"Accept": {"application/json"}}
	entry.ResponseBody = []byte("body")
	r.Record(entry)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	// Shadow traffic finishing late, exactly like shadow.Shadower.
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				entry.SetShadowResults(shadowResults())
			}
		}
	}()

	for i := 0; i < 300; i++ {
		// handleRequestDetail: writeJSON(w, 200, rec.Get(id))
		data, err := json.Marshal(r.Get(entry.ID))
		if err != nil {
			t.Fatalf("json.Marshal(Get()) error = %v", err)
		}
		if !strings.Contains(string(data), "\"id\"") {
			t.Fatalf("marshalled entry lost its id: %s", data)
		}

		// handleSessions / handleAPIMap: rec.All()
		for _, e := range r.All() {
			_ = e.Summary()
		}

		// broadcastEntry: entry.Summary()
		_ = entry.Summary()
	}

	close(stop)
	wg.Wait()

	if got := len(entry.GetShadowResults()); got != 2 {
		t.Errorf("shadow results len = %d, want 2", got)
	}
}

// TestRecorderRecordRaceWithReads checks the recorder ring buffer against
// concurrent recording, eviction and reads. Run with -race to get the full value
// of it.
func TestRecorderRecordRaceWithReads(t *testing.T) {
	r := New(8)
	r.OnChange(func(e *Entry) { _ = e.Summary() })

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			e := makeEntry("GET", "/write", 200, false)
			e.SetShadowResults(shadowResults())
			r.Record(e)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			for _, e := range r.All() {
				_, _ = json.Marshal(e)
			}
			_ = r.List(Filter{Limit: 10})
			_ = r.Count()
		}
	}()
	wg.Wait()

	if got := r.Count(); got != 8 {
		t.Errorf("Count() = %d, want 8 (ring buffer limit)", got)
	}
}
