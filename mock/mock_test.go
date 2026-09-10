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
