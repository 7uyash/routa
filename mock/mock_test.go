package mock

import (
	"testing"
	"time"

	"github.com/7uyash/routa/recorder"
)

func TestMockLabCreateAndMatch(t *testing.T) {
	lab := NewLab()

	rule := lab.CreateRule("Test Mock", "GET", "/api/v1/users", 200, nil, `{"users": []}`, 0, true)
	if rule.ID == "" {
		t.Fatal("expected non-empty rule ID")
	}

	matched := lab.MatchRequest("GET", "/api/v1/users")
	if matched == nil {
		t.Fatal("expected matched mock rule")
	}
	if matched.Status != 200 {
		t.Errorf("expected status 200, got %d", matched.Status)
	}

	status, _, body := matched.ServeMock()
	if status != 200 || string(body) != `{"users": []}` {
		t.Errorf("unexpected served mock output: status=%d body=%s", status, string(body))
	}
}

func TestCreateFromRequest(t *testing.T) {
	lab := NewLab()

	entry := &recorder.Entry{
		ID:           "req_123",
		Method:       "POST",
		Path:         "/api/v1/orders/456",
		StatusCode:   201,
		ResponseBody: []byte(`{"status": "created"}`),
		Timestamp:    time.Now(),
	}

	rule := lab.CreateFromRequest(entry)
	if rule == nil {
		t.Fatal("expected created mock rule from entry")
	}
	if rule.Status != 201 {
		t.Errorf("expected status 201, got %d", rule.Status)
	}
	if rule.Body != `{"status": "created"}` {
		t.Errorf("unexpected body: %s", rule.Body)
	}
}

// TestMockUsesCanonicalMatching pins the mock lab to traffic.MatchPath. The
// mock lab used to have its own matcher that required a non-empty method and
// path, so a rule created without either of them never fired.
func TestMockUsesCanonicalMatching(t *testing.T) {
	pathCases := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"exact match", "/api/v1/users", "/api/v1/users", true},
		{"exact match does not spill over", "/api/v1/users", "/api/v1/users/1", false},
		{"segment wildcard", "/api/*", "/api/v1", true},
		{"segment wildcard matches the prefix", "/api/*", "/api", true},
		{"segment wildcard respects the boundary", "/api/*", "/apifoo", false},
		{"string prefix wildcard", "/api*", "/apifoo", true},
		{"bare star matches any path", "*", "/anything", true},
		{"empty path matches any path", "", "/anything", true},
	}

	for _, tt := range pathCases {
		t.Run(tt.name, func(t *testing.T) {
			lab := NewLab()
			lab.CreateRule("probe", "GET", tt.pattern, 200, nil, "", 0, true)
			if got := lab.MatchRequest("GET", tt.path) != nil; got != tt.want {
				t.Errorf("MatchRequest(GET, %q) with path %q = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}

	methodCases := []struct {
		name   string
		method string
		want   bool
	}{
		{"exact method", "GET", true},
		{"different method", "DELETE", false},
		{"star matches any method", "*", true},
		{"empty method matches any method", "", true},
	}

	for _, tt := range methodCases {
		t.Run(tt.name, func(t *testing.T) {
			lab := NewLab()
			lab.CreateRule("probe", tt.method, "/api/*", 200, nil, "", 0, true)
			if got := lab.MatchRequest("GET", "/api/users") != nil; got != tt.want {
				t.Errorf("MatchRequest(GET, /api/users) with method %q = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}
