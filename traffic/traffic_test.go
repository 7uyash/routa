package traffic_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/7uyash/routa/traffic"
)

func TestTimingJSONSerialization(t *testing.T) {
	timing := &traffic.Timing{
		DNSLookup:    10 * time.Millisecond,
		TCPConnect:   20 * time.Millisecond,
		TLSHandshake: 30 * time.Millisecond,
		FirstByte:    40 * time.Millisecond,
		Total:        100 * time.Millisecond,
	}

	data, err := json.Marshal(timing)
	if err != nil {
		t.Fatalf("failed to marshal Timing: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	expectedKeys := []string{"dns_lookup_ms", "tcp_connect_ms", "tls_handshake_ms", "first_byte_ms", "total_ms"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q missing from serialized payload", key)
		}
	}
}
