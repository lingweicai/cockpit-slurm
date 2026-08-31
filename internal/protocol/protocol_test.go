package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	msg := NewEnvelope("MSG001", MessagePing, json.RawMessage(`{}`))

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	dec := NewDecoder(&buf)
	got, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode() returned error: %v", err)
	}

	if got.Protocol != ProtocolName {
		t.Fatalf("Protocol = %q, want %q", got.Protocol, ProtocolName)
	}
	if got.Version != ProtocolVersion {
		t.Fatalf("Version = %q, want %q", got.Version, ProtocolVersion)
	}
	if got.MessageID != "MSG001" {
		t.Fatalf("MessageID = %q, want %q", got.MessageID, "MSG001")
	}
	if got.Type != MessagePing {
		t.Fatalf("Type = %q, want %q", got.Type, MessagePing)
	}
	if !bytes.Equal(got.Payload, json.RawMessage(`{}`)) {
		t.Fatalf("Payload = %s, want %s", got.Payload, json.RawMessage(`{}`))
	}
}

func TestDecoderRejectsZeroLengthFrame(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00})

	dec := NewDecoder(&buf)
	if _, err := dec.Decode(); err == nil {
		t.Fatal("Decode() accepted a zero-length payload")
	}
}

func TestDecoderRejectsIncompletePayload(t *testing.T) {
	var buf bytes.Buffer
	// length = 16, but payload is only 8 bytes
	buf.Write([]byte{0x00, 0x00, 0x00, 0x10})
	buf.Write([]byte("12345678"))

	dec := NewDecoder(&buf)
	if _, err := dec.Decode(); err == nil {
		t.Fatal("Decode() accepted a truncated payload")
	}
}

func TestDecoderRejectsInvalidUTF8JSON(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte{0xff, 0xfe, 0xfd}
	length := uint32(len(payload))
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], length)
	buf.Write(header[:])
	buf.Write(payload)

	dec := NewDecoder(&buf)
	if _, err := dec.Decode(); err == nil {
		t.Fatal("Decode() accepted invalid UTF-8 JSON")
	}
}

func TestValidateRejectsUnknownMessageType(t *testing.T) {
	e := Envelope{
		Protocol:  ProtocolName,
		Version:   ProtocolVersion,
		MessageID: "MSG001",
		Type:      MessageType("not-real"),
		Payload:   json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted an unknown message type")
	}
}

func TestValidateRejectsMissingMessageID(t *testing.T) {
	e := Envelope{
		Protocol: ProtocolName,
		Version:  ProtocolVersion,
		Type:     MessagePing,
		Payload:  json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted an empty messageId")
	}
}

func TestValidateRejectsMissingPayload(t *testing.T) {
	e := Envelope{
		Protocol:  ProtocolName,
		Version:   ProtocolVersion,
		MessageID: "MSG001",
		Type:      MessagePing,
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted a missing payload")
	}
}

func TestEncoderWritesFullFrame(t *testing.T) {
	msg := NewEnvelope("MSG001", MessageHello, json.RawMessage(`{"client":"test"}`))
	var buf bytes.Buffer

	enc := NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	if len(buf.Bytes()) < 4 {
		t.Fatal("encoded frame too short")
	}

	length := binary.BigEndian.Uint32(buf.Bytes()[:4])
	if int(length) != len(buf.Bytes())-4 {
		t.Fatalf("length prefix = %d, actual payload length = %d", length, len(buf.Bytes())-4)
	}

	if !strings.Contains(buf.String(), "\"type\":\"hello\"") {
		t.Fatal("encoded payload did not contain hello type")
	}
}
