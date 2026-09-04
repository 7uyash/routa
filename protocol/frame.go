// Package protocol defines the binary wire format for the Routa tunnel.
//
// Every message between agent and relay is a Frame:
//
//	Type (1 byte) | RequestID (4 bytes) | PayloadLen (4 bytes) | Payload (N bytes)
//
// This allows multiplexing many concurrent HTTP request/response pairs over
// a single WebSocket connection.
package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Frame types exchanged over the tunnel.
const (
	TypeHTTPRequest  uint8 = 1
	TypeHTTPResponse uint8 = 2
	TypePing         uint8 = 10
	TypePong         uint8 = 11
	TypeAuth         uint8 = 20
	TypeAuthOK       uint8 = 21
	TypeAuthFail     uint8 = 22
	TypeTunnelReady  uint8 = 30
	TypeError        uint8 = 40
)

// frameHeaderSize is the fixed size of a frame header: type(1) + requestID(4) + payloadLen(4).
const frameHeaderSize = 9

// MaxPayloadSize is the maximum allowed payload to prevent memory exhaustion.
const MaxPayloadSize = 10 * 1024 * 1024 // 10 MB

// Frame represents a single protocol message.
type Frame struct {
	Type       uint8
	RequestID  uint32
	PayloadLen uint32
	Payload    []byte
}

// NewFrame creates a frame with the given type, request ID, and payload.
func NewFrame(typ uint8, requestID uint32, payload []byte) Frame {
	return Frame{
		Type:       typ,
		RequestID:  requestID,
		PayloadLen: uint32(len(payload)),
		Payload:    payload,
	}
}

// Encode writes the frame to the given writer in binary format.
func (f *Frame) Encode(w io.Writer) error {
	header := make([]byte, frameHeaderSize)
	header[0] = f.Type
	binary.BigEndian.PutUint32(header[1:5], f.RequestID)
	binary.BigEndian.PutUint32(header[5:9], f.PayloadLen)

	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("write frame header: %w", err)
	}
	if f.PayloadLen > 0 {
		if _, err := w.Write(f.Payload); err != nil {
			return fmt.Errorf("write frame payload: %w", err)
		}
	}
	return nil
}

// DecodeFrame reads a single frame from the reader.
func DecodeFrame(r io.Reader) (Frame, error) {
	header := make([]byte, frameHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return Frame{}, fmt.Errorf("read frame header: %w", err)
	}

	f := Frame{
		Type:       header[0],
		RequestID:  binary.BigEndian.Uint32(header[1:5]),
		PayloadLen: binary.BigEndian.Uint32(header[5:9]),
	}

	if f.PayloadLen > MaxPayloadSize {
		return Frame{}, fmt.Errorf("payload size %d exceeds max %d", f.PayloadLen, MaxPayloadSize)
	}

	if f.PayloadLen > 0 {
		f.Payload = make([]byte, f.PayloadLen)
		if _, err := io.ReadFull(r, f.Payload); err != nil {
			return Frame{}, fmt.Errorf("read frame payload: %w", err)
		}
	}
	return f, nil
}

// TypeName returns a human-readable name for a frame type.
func TypeName(t uint8) string {
	switch t {
	case TypeHTTPRequest:
		return "HTTPRequest"
	case TypeHTTPResponse:
		return "HTTPResponse"
	case TypePing:
		return "Ping"
	case TypePong:
		return "Pong"
	case TypeAuth:
		return "Auth"
	case TypeAuthOK:
		return "AuthOK"
	case TypeAuthFail:
		return "AuthFail"
	case TypeTunnelReady:
		return "TunnelReady"
	case TypeError:
		return "Error"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}
