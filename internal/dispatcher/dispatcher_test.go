package dispatcher

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/protocol"
)

type authenticatedHandlerSpy struct {
	calls int
	user  connection.UserContext
}

func (h *authenticatedHandlerSpy) HandleAuthenticated(_ context.Context, user connection.UserContext, message protocol.Envelope) protocol.Envelope {
	h.calls++
	h.user = user
	return protocol.NewEnvelope(message.MessageID, protocol.MessageCapabilitiesResponse, json.RawMessage(`{"capabilities":{"nodes.drain":false,"nodes.resume":false}}`))
}

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
	if resp.MessageID != msg.MessageID {
		t.Fatalf("MessageID = %q, want %q", resp.MessageID, msg.MessageID)
	}
	if err := protocol.ValidateEnvelope(resp); err != nil {
		t.Fatalf("ValidateEnvelope(response) returned error: %v", err)
	}
}

func TestDispatchKnownUnimplementedTypeReturnsNotImplemented(t *testing.T) {
	d := NewDispatcher()
	msg := protocol.NewEnvelope("MSG003", protocol.MessageCapabilitiesResponse, json.RawMessage(`{"capabilities":{"nodes.drain":false,"nodes.resume":false}}`))

	resp := d.Dispatch(context.Background(), msg)

	if resp.Type != protocol.MessageError {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessageError)
	}
	if resp.MessageID != msg.MessageID {
		t.Fatalf("MessageID = %q, want %q", resp.MessageID, msg.MessageID)
	}
	var errorPayload protocol.ErrorPayload
	if err := json.Unmarshal(resp.Payload, &errorPayload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if errorPayload.Code != protocol.ErrorCodeUnsupportedMessageType || errorPayload.Category != protocol.ErrorCategoryProtocol || errorPayload.Retryable {
		t.Fatalf("error payload = %#v, want canonical unsupported-message-type", errorPayload)
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

func TestAuthenticatedDispatchRequiresAndPassesConnectionIdentity(t *testing.T) {
	d := NewDispatcher()
	handler := &authenticatedHandlerSpy{}
	if err := d.RegisterAuthenticated(protocol.MessageCapabilitiesRequest, handler); err != nil {
		t.Fatalf("RegisterAuthenticated() returned error: %v", err)
	}
	message := protocol.NewEnvelope("CAP-1", protocol.MessageCapabilitiesRequest, json.RawMessage(`{}`))
	user := connection.UserContext{UID: 1008, GID: 978, Username: "slurm_admin"}

	response := d.DispatchWithUser(context.Background(), user, message)
	if response.Type != protocol.MessageCapabilitiesResponse || response.MessageID != message.MessageID {
		t.Fatalf("response = %#v, want correlated capabilities response", response)
	}
	if handler.calls != 1 || handler.user.UID != user.UID || handler.user.GID != user.GID || handler.user.Username != user.Username {
		t.Fatalf("handler calls=%d user=%#v, want one call with server identity %#v", handler.calls, handler.user, user)
	}

	withoutIdentity := d.Dispatch(context.Background(), message)
	if withoutIdentity.Type != protocol.MessageError || handler.calls != 1 {
		t.Fatalf("unauthenticated dispatch response=%#v handler calls=%d, want error and no handler call", withoutIdentity, handler.calls)
	}
	var errorPayload protocol.ErrorPayload
	if err := json.Unmarshal(withoutIdentity.Payload, &errorPayload); err != nil {
		t.Fatalf("decode identity error: %v", err)
	}
	if errorPayload.Code != protocol.ErrorCodeIdentityUnavailable || errorPayload.Category != protocol.ErrorCategoryAuthentication || errorPayload.Retryable {
		t.Fatalf("identity error payload = %#v", errorPayload)
	}
}
