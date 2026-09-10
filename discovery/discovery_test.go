package discovery

import (
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/v1/users/123", "/api/v1/users/{id}"},
		{"/api/v1/orders/7b2e987c-3f4a-4b1a-9f5a-8b3c4d5e6f7a/items", "/api/v1/orders/{id}/items"},
		{"/auth/login", "/auth/login"},
		{"/posts/507f1f77bcf86cd799439011", "/posts/{id}"},
	}

	for _, tt := range tests {
		got := NormalizePath(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestInferFriendlyName(t *testing.T) {
	name := inferFriendlyName(3000, "Express", "Node.js", "application/json")
	if name != "Node.js / Express App (:3000)" {
		t.Errorf("unexpected friendly name: %s", name)
	}
}
