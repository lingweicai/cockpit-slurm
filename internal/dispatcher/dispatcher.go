package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
)

// Handler processes one message type and returns a response envelope.
type Handler interface {
	Handle(ctx context.Context, msg protocol.Envelope) protocol.Envelope
}

// Dispatcher routes inbound envelopes to their handlers.
type Dispatcher interface {
	Dispatch(ctx context.Context, msg protocol.Envelope) protocol.Envelope
}

// MessageDispatcher routes protocol messages to registered handlers.
type MessageDispatcher struct {
	handlers map[protocol.MessageType]Handler
	counter  atomic.Uint64
}

// NewDispatcher creates a dispatcher with the Phase 1D handlers.
func NewDispatcher() *MessageDispatcher {
	d := &MessageDispatcher{
		handlers: make(map[protocol.MessageType]Handler),
	}
	d.register(protocol.MessageHello, helloHandler{dispatcher: d})
	d.register(protocol.MessagePing, pingHandler{dispatcher: d})
	return d
}

func (d *MessageDispatcher) register(t protocol.MessageType, h Handler) {
	d.handlers[t] = h
}

// Dispatch routes an inbound message to a registered handler.
// If the message type is known but unimplemented, it returns a NOT_IMPLEMENTED error.
// If the message type is unknown, it returns an UNKNOWN_MESSAGE_TYPE error.
func (d *MessageDispatcher) Dispatch(ctx context.Context, msg protocol.Envelope) protocol.Envelope {
	if msg.Protocol != protocol.ProtocolName || msg.Version != protocol.ProtocolVersion {
		return d.errorEnvelope("INVALID_MESSAGE", "invalid protocol envelope")
	}
	if msg.MessageID == "" || msg.Type == "" || len(msg.Payload) == 0 {
		return d.errorEnvelope("INVALID_MESSAGE", "invalid protocol envelope")
	}

	h, ok := d.handlers[msg.Type]
	if !ok {
		if msg.Type.IsKnown() {
			return d.errorEnvelope("NOT_IMPLEMENTED", fmt.Sprintf("message type %q is not implemented", msg.Type))
		}
		return d.errorEnvelope("UNKNOWN_MESSAGE_TYPE", fmt.Sprintf("unknown message type %q", msg.Type))
	}

	return h.Handle(ctx, msg)
}

func (d *MessageDispatcher) nextMessageID() string {
	return fmt.Sprintf("MSG%06d", d.counter.Add(1))
}

func (d *MessageDispatcher) errorEnvelope(code, message string) protocol.Envelope {
	payload, err := json.Marshal(map[string]string{
		"code":    code,
		"message": message,
	})
	if err != nil {
		payload = []byte(`{"code":"INTERNAL_ERROR","message":"failed to encode error payload"}`)
	}
	return protocol.NewEnvelope(d.nextMessageID(), protocol.MessageError, payload)
}

type helloHandler struct {
	dispatcher *MessageDispatcher
}

func (h helloHandler) Handle(ctx context.Context, msg protocol.Envelope) protocol.Envelope {
	payload, err := json.Marshal(map[string]string{
		"server":        "cockpit-slurm-bridge",
		"serverVersion": "0.1.0",
	})
	if err != nil {
		return h.dispatcher.errorEnvelope("HANDLER_ERROR", "hello handler failed to encode response")
	}
	return protocol.NewEnvelope(h.dispatcher.nextMessageID(), protocol.MessageHelloResponse, payload)
}

type pingHandler struct {
	dispatcher *MessageDispatcher
}

func (h pingHandler) Handle(ctx context.Context, msg protocol.Envelope) protocol.Envelope {
	payload, err := json.Marshal(map[string]string{
		"status": "ok",
	})
	if err != nil {
		return h.dispatcher.errorEnvelope("HANDLER_ERROR", "ping handler failed to encode response")
	}
	return protocol.NewEnvelope(h.dispatcher.nextMessageID(), protocol.MessagePong, payload)
}
