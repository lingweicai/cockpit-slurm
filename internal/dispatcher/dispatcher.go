package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
	"github.com/lingweicai/cockpit-slurm/internal/service"
)

// Handler processes one message type and returns a response envelope.
type Handler interface {
	Handle(ctx context.Context, msg protocol.Envelope) protocol.Envelope
}

type AuthenticatedHandler interface {
	HandleAuthenticated(ctx context.Context, user connection.UserContext, msg protocol.Envelope) protocol.Envelope
}

// Dispatcher routes inbound envelopes to their handlers.
type Dispatcher interface {
	Dispatch(ctx context.Context, msg protocol.Envelope) protocol.Envelope
}

// MessageDispatcher routes protocol messages to registered handlers.
type MessageDispatcher struct {
	handlers              map[protocol.MessageType]Handler
	authenticatedHandlers map[protocol.MessageType]AuthenticatedHandler
	counter               atomic.Uint64
}

// NewDispatcher creates a dispatcher with the Phase 1D handlers.
func NewDispatcher() *MessageDispatcher {
	return NewDispatcherWithCache(resource.NewNodeCache())
}

// NewDispatcherWithCache creates a dispatcher backed by the supplied Node cache.
func NewDispatcherWithCache(cache *resource.NodeCache) *MessageDispatcher {
	return NewDispatcherWithCacheAndCapabilities(cache, nil)
}

func NewDispatcherWithCacheAndCapabilities(cache *resource.NodeCache, resolver service.CapabilityContextResolver) *MessageDispatcher {
	if cache == nil {
		cache = resource.NewNodeCache()
	}

	d := &MessageDispatcher{
		handlers:              make(map[protocol.MessageType]Handler),
		authenticatedHandlers: make(map[protocol.MessageType]AuthenticatedHandler),
	}
	d.register(protocol.MessageHello, helloHandler{dispatcher: d})
	d.register(protocol.MessagePing, pingHandler{dispatcher: d})
	d.register(protocol.MessageQuery, service.NewQueryHandler(service.NewNodeQueryService(cache)))
	_ = d.RegisterAuthenticated(protocol.MessageCapabilitiesRequest, service.NewCapabilitiesHandler(resolver))
	return d
}

func (d *MessageDispatcher) register(t protocol.MessageType, h Handler) {
	d.handlers[t] = h
}

func (d *MessageDispatcher) Register(t protocol.MessageType, h Handler) error {
	if h == nil {
		return fmt.Errorf("handler is nil")
	}
	d.handlers[t] = h
	return nil
}

func (d *MessageDispatcher) RegisterAuthenticated(t protocol.MessageType, h AuthenticatedHandler) error {
	if h == nil {
		return fmt.Errorf("authenticated handler is nil")
	}
	d.authenticatedHandlers[t] = h
	return nil
}

// Dispatch routes an inbound envelope that does not require connection identity.
func (d *MessageDispatcher) Dispatch(ctx context.Context, msg protocol.Envelope) protocol.Envelope {
	return d.dispatch(ctx, nil, msg)
}

func (d *MessageDispatcher) DispatchWithUser(ctx context.Context, user connection.UserContext, msg protocol.Envelope) protocol.Envelope {
	return d.dispatch(ctx, &user, msg)
}

func (d *MessageDispatcher) dispatch(ctx context.Context, user *connection.UserContext, msg protocol.Envelope) protocol.Envelope {
	if msg.Protocol != protocol.ProtocolName || msg.Version != protocol.ProtocolVersion {
		return d.errorEnvelope(msg.MessageID, protocol.ErrorCodeInvalidPayload, protocol.ErrorCategoryValidation, "Invalid protocol envelope", false)
	}
	if msg.MessageID == "" || msg.Type == "" || len(msg.Payload) == 0 {
		return d.errorEnvelope(msg.MessageID, protocol.ErrorCodeInvalidPayload, protocol.ErrorCategoryValidation, "Invalid protocol envelope", false)
	}

	if h, ok := d.authenticatedHandlers[msg.Type]; ok {
		if user == nil || user.Username == "" {
			return d.errorEnvelope(msg.MessageID, protocol.ErrorCodeIdentityUnavailable, protocol.ErrorCategoryAuthentication, "Unable to determine the authenticated user", false)
		}
		return h.HandleAuthenticated(ctx, *user, msg)
	}

	h, ok := d.handlers[msg.Type]
	if !ok {
		return d.errorEnvelope(msg.MessageID, protocol.ErrorCodeUnsupportedMessageType, protocol.ErrorCategoryProtocol, "Message type is not supported", false)
	}

	return h.Handle(ctx, msg)
}

func (d *MessageDispatcher) nextMessageID() string {
	return fmt.Sprintf("MSG%06d", d.counter.Add(1))
}

func (d *MessageDispatcher) errorEnvelope(messageID string, code protocol.ErrorCode, category protocol.ErrorCategory, message string, retryable bool) protocol.Envelope {
	if messageID == "" {
		messageID = d.nextMessageID()
	}
	return protocol.NewErrorEnvelope(messageID, protocol.ErrorPayload{
		Code:      code,
		Category:  category,
		Message:   message,
		Retryable: retryable,
	})
}

type helloHandler struct {
	dispatcher *MessageDispatcher
}

func (h helloHandler) Handle(ctx context.Context, msg protocol.Envelope) protocol.Envelope {
	payload, err := json.Marshal(map[string]string{
		"server":        "cockpit-slurm",
		"serverVersion": "0.1.0",
	})
	if err != nil {
		return h.dispatcher.errorEnvelope(msg.MessageID, protocol.ErrorCodeInternal, protocol.ErrorCategoryInternal, "Internal error", true)
	}
	return protocol.NewEnvelope(msg.MessageID, protocol.MessageHelloResponse, payload)
}

type pingHandler struct {
	dispatcher *MessageDispatcher
}

func (h pingHandler) Handle(ctx context.Context, msg protocol.Envelope) protocol.Envelope {
	payload, err := json.Marshal(map[string]string{
		"status": "ok",
	})
	if err != nil {
		return h.dispatcher.errorEnvelope(msg.MessageID, protocol.ErrorCodeInternal, protocol.ErrorCategoryInternal, "Internal error", true)
	}
	return protocol.NewEnvelope(msg.MessageID, protocol.MessagePong, payload)
}
