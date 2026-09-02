package dispatcher

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
)

func TestDispatchHelloReturnsHelloResponse(t *testing.T) {
	d := NewDispatcher()
	msg := protocol.NewEnvelope(
		"MSG001",
		protocol.MessageHello,
		json.RawMessage(`{"client":"cockpit-slurm-channel","clientVersion":"0.1.0"}`),
	)

	resp := d.Dispatch(context.Background(), msg)

	if resp.Type != protocol.MessageHelloResponse {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessageHelloResponse)
	}
	if resp.Protocol != protocol.ProtocolName {
		t.Fatalf("Protocol = %q, want %q", resp.Protocol, protocol.ProtocolName)
	}
	if resp.Version != protocol.ProtocolVersion {
		t.Fatalf("Version = %q, want %q", resp.Version, protocol.ProtocolVersion)
	}
	if resp.MessageID == "" {
		t.Fatal("MessageID is empty")
	}
	if len(resp.Payload) == 0 {
		t.Fatal("Payload is empty")
	}
	if err := protocol.ValidateEnvelope(resp); err != nil {
		t.Fatalf("ValidateEnvelope(response) returned error: %v", err)
	}
}

func TestDispatchPingReturnsPong(t *testing.T) {
	d := NewDispatcher()
	msg := protocol.NewEnvelope("MSG002", protocol.MessagePing, json.RawMessage(`{"value":1}`))

	resp := d.Dispatch(context.Background(), msg)

	if resp.Type != protocol.MessagePong {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessagePong)
	}
	if resp.MessageID == "" {
		t.Fatal("MessageID is empty")
	}
	if err := protocol.ValidateEnvelope(resp); err != nil {
		t.Fatalf("ValidateEnvelope(response) returned error: %v", err)
	}
}

func TestDispatchKnownUnimplementedTypeReturnsNotImplemented(t *testing.T) {
	d := NewDispatcher()
	msg := protocol.NewEnvelope("MSG003", protocol.MessageQuery, json.RawMessage(`{"resource":"jobs"}`))

	resp := d.Dispatch(context.Background(), msg)

	if resp.Type != protocol.MessageError {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessageError)
	}
	if err := protocol.ValidateEnvelope(resp); err != nil {
		t.Fatalf("ValidateEnvelope(response) returned error: %v", err)
	}
}

func TestDispatchUnknownTypeReturnsUnknownMessageType(t *testing.T) {
	d := NewDispatcher()
	msg := protocol.NewEnvelope("MSG004", protocol.MessageType("not-real"), json.RawMessage(`{"value":1}`))

	resp := d.Dispatch(context.Background(), msg)

	if resp.Type != protocol.MessageError {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessageError)
	}
	if err := protocol.ValidateEnvelope(resp); err != nil {
		t.Fatalf("ValidateEnvelope(response) returned error: %v", err)
	}
}
