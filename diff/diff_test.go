package diff

import (
	"testing"

	"github.com/7uyash/routa/recorder"
)

func TestCompareIdentical(t *testing.T) {
	orig := &recorder.Entry{
		StatusCode:      200,
		ResponseHeaders: map[string][]string{"Content-Type": {"application/json"}},
		ResponseBody:    []byte(`{"status":"ok"}`),
	}
	replay := &recorder.Entry{
		StatusCode:      200,
		ResponseHeaders: map[string][]string{"Content-Type": {"application/json"}},
		ResponseBody:    []byte(`{"status":"ok"}`),
	}

	res := Compare(orig, replay)
	if res.StatusDiff != "" {
		t.Errorf("StatusDiff: got %q, want empty", res.StatusDiff)
	}
	if len(res.HeadersDiff) != 0 {
		t.Errorf("HeadersDiff should be empty, got %v", res.HeadersDiff)
	}
	if res.BodyDiff != "" {
		t.Errorf("BodyDiff should be empty, got %q", res.BodyDiff)
	}
}

func TestCompareStatusDiff(t *testing.T) {
	orig := &recorder.Entry{StatusCode: 200}
	replay := &recorder.Entry{StatusCode: 500}

	res := Compare(orig, replay)
	if res.StatusDiff != "200 -> 500" {
		t.Errorf("StatusDiff: got %q, want 200 -> 500", res.StatusDiff)
	}
}

func TestCompareHeaderDiff(t *testing.T) {
	orig := &recorder.Entry{
		ResponseHeaders: map[string][]string{
			"Content-Type": {"text/plain"},
			"X-Trace":      {"123"},
		},
	}
	replay := &recorder.Entry{
		ResponseHeaders: map[string][]string{
			"Content-Type": {"application/json"},
			"X-New":        {"456"},
		},
	}

	res := Compare(orig, replay)

	if got := res.HeadersDiff["Content-Type"]; got != "changed: text/plain -> application/json" {
		t.Errorf("Content-Type: got %q", got)
	}
	if got := res.HeadersDiff["X-Trace"]; got != "removed (was: 123)" {
		t.Errorf("X-Trace: got %q", got)
	}
	if got := res.HeadersDiff["X-New"]; got != "added: 456" {
		t.Errorf("X-New: got %q", got)
	}
}

func TestCompareJSONBodyDiff(t *testing.T) {
	orig := &recorder.Entry{
		ResponseBody: []byte(`{"user":{"name":"alice","role":"admin"}, "active":true}`),
	}
	replay := &recorder.Entry{
		ResponseBody: []byte(`{"user":{"name":"alice","role":"user"}, "active":true, "new_field":1}`),
	}

	res := Compare(orig, replay)
	if res.BodyDiff == "" {
		t.Fatal("BodyDiff should not be empty")
	}
	if !contains(res.BodyDiff, "~ user.role: admin -> user") {
		t.Errorf("BodyDiff missing change: %s", res.BodyDiff)
	}
	if !contains(res.BodyDiff, "+ new_field: added (1)") {
		t.Errorf("BodyDiff missing addition: %s", res.BodyDiff)
	}
}

func TestCompareNonJSONBodyDiff(t *testing.T) {
	orig := &recorder.Entry{
		ResponseBody: []byte(`hello world`),
	}
	replay := &recorder.Entry{
		ResponseBody: []byte(`hello routa`),
	}

	res := Compare(orig, replay)
	if res.BodyDiff != "content changed (same length)" {
		t.Errorf("BodyDiff: got %q", res.BodyDiff)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && (s[:len(sub)] == sub || s[len(s)-len(sub):] == sub || func() bool {
		for i := 1; i < len(s)-len(sub)+1; i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()))
}
