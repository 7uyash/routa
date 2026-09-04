package protocol

import (
	"bytes"
	"testing"
)

func TestFrameEncodeDecode(t *testing.T) {
	tests := []struct {
		name      string
		frameType uint8
		requestID uint32
		payload   []byte
	}{
		{
			name:      "ping no payload",
			frameType: TypePing,
			requestID: 0,
			payload:   nil,
		},
		{
			name:      "http request with payload",
			frameType: TypeHTTPRequest,
			requestID: 42,
			payload:   []byte(`{"method":"GET","url":"/test"}`),
		},
		{
			name:      "http response with large payload",
			frameType: TypeHTTPResponse,
			requestID: 9999,
			payload:   bytes.Repeat([]byte("x"), 4096),
		},
		{
			name:      "auth frame",
			frameType: TypeAuth,
			requestID: 0,
			payload:   []byte(`{"token":"secret123"}`),
		},
		{
			name:      "empty payload frame",
			frameType: TypePong,
			requestID: 1,
			payload:   []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NewFrame(tt.frameType, tt.requestID, tt.payload)

			// Encode
			var buf bytes.Buffer
			if err := original.Encode(&buf); err != nil {
				t.Fatalf("encode: %v", err)
			}

			// Decode
			decoded, err := DecodeFrame(&buf)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			// Verify
			if decoded.Type != tt.frameType {
				t.Errorf("type: got %d, want %d", decoded.Type, tt.frameType)
			}
			if decoded.RequestID != tt.requestID {
				t.Errorf("requestID: got %d, want %d", decoded.RequestID, tt.requestID)
			}
			if !bytes.Equal(decoded.Payload, tt.payload) {
				t.Errorf("payload mismatch: got %d bytes, want %d bytes",
					len(decoded.Payload), len(tt.payload))
			}
		})
	}
}

func TestDecodeFramePayloadTooLarge(t *testing.T) {
	// Create a frame header with an enormous payload size.
	header := make([]byte, frameHeaderSize)
	header[0] = TypeHTTPRequest
	// Set payload length to MaxPayloadSize + 1
	size := uint32(MaxPayloadSize + 1)
	header[5] = byte(size >> 24)
	header[6] = byte(size >> 16)
	header[7] = byte(size >> 8)
	header[8] = byte(size)

	_, err := DecodeFrame(bytes.NewReader(header))
	if err == nil {
		t.Fatal("expected error for oversized payload")
	}
}

func TestTypeName(t *testing.T) {
	tests := []struct {
		t    uint8
		want string
	}{
		{TypeHTTPRequest, "HTTPRequest"},
		{TypeHTTPResponse, "HTTPResponse"},
		{TypePing, "Ping"},
		{TypePong, "Pong"},
		{TypeAuth, "Auth"},
		{TypeAuthOK, "AuthOK"},
		{TypeAuthFail, "AuthFail"},
		{TypeTunnelReady, "TunnelReady"},
		{TypeError, "Error"},
		{255, "Unknown(255)"},
	}

	for _, tt := range tests {
		got := TypeName(tt.t)
		if got != tt.want {
			t.Errorf("TypeName(%d) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestMessageEncodeDecode(t *testing.T) {
	// Test HTTPRequestMsg round-trip.
	reqMsg := HTTPRequestMsg{
		Method:  "POST",
		URL:     "/api/test?foo=bar",
		Path:    "/api/test",
		Query:   "foo=bar",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{"key":"value"}`),
		Host:    "example.com",
	}

	frame, err := EncodePayload(TypeHTTPRequest, 1, reqMsg)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	if frame.Type != TypeHTTPRequest {
		t.Errorf("frame type: got %d, want %d", frame.Type, TypeHTTPRequest)
	}

	var decoded HTTPRequestMsg
	if err := DecodePayload(frame, &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if decoded.Method != reqMsg.Method {
		t.Errorf("method: got %q, want %q", decoded.Method, reqMsg.Method)
	}
	if decoded.Path != reqMsg.Path {
		t.Errorf("path: got %q, want %q", decoded.Path, reqMsg.Path)
	}
	if decoded.Host != reqMsg.Host {
		t.Errorf("host: got %q, want %q", decoded.Host, reqMsg.Host)
	}
}

func TestMultipleFramesInStream(t *testing.T) {
	var buf bytes.Buffer

	// Write 3 frames.
	for i := uint32(0); i < 3; i++ {
		f := NewFrame(TypeHTTPRequest, i, []byte("hello"))
		if err := f.Encode(&buf); err != nil {
			t.Fatalf("encode frame %d: %v", i, err)
		}
	}

	// Read them back.
	for i := uint32(0); i < 3; i++ {
		f, err := DecodeFrame(&buf)
		if err != nil {
			t.Fatalf("decode frame %d: %v", i, err)
		}
		if f.RequestID != i {
			t.Errorf("frame %d: requestID = %d, want %d", i, f.RequestID, i)
		}
		if string(f.Payload) != "hello" {
			t.Errorf("frame %d: payload = %q, want %q", i, f.Payload, "hello")
		}
	}
}
