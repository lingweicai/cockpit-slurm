package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/identity"
	"github.com/lingweicai/cockpit-slurm/internal/protocol"
)

type CapabilityContextResolver interface {
	Resolve(ctx context.Context, requester connection.UserContext) (connection.UserContext, error)
}

type CapabilitiesHandler struct {
	resolver CapabilityContextResolver
}

func NewCapabilitiesHandler(resolver CapabilityContextResolver) *CapabilitiesHandler {
	return &CapabilitiesHandler{resolver: resolver}
}

func (h *CapabilitiesHandler) HandleAuthenticated(ctx context.Context, requester connection.UserContext, message protocol.Envelope) protocol.Envelope {
	if _, err := protocol.DecodeCapabilitiesRequestPayload(message.Payload); err != nil {
		return protocol.NewErrorEnvelope(message.MessageID, protocol.ErrorPayload{
			Code:      protocol.ErrorCodeInvalidPayload,
			Category:  protocol.ErrorCategoryValidation,
			Message:   "Invalid capabilities request",
			Retryable: false,
		})
	}
	if requester.Username == "" {
		return protocol.NewErrorEnvelope(message.MessageID, protocol.ErrorPayload{
			Code:      protocol.ErrorCodeIdentityUnavailable,
			Category:  protocol.ErrorCategoryAuthentication,
			Message:   "Unable to determine the authenticated user",
			Retryable: false,
		})
	}
	if h == nil || h.resolver == nil {
		return capabilityUnavailable(message.MessageID)
	}

	resolved, err := h.resolver.Resolve(ctx, requester)
	if errors.Is(err, identity.ErrIdentityUnavailable) {
		return protocol.NewErrorEnvelope(message.MessageID, protocol.ErrorPayload{
			Code:      protocol.ErrorCodeIdentityUnavailable,
			Category:  protocol.ErrorCategoryAuthentication,
			Message:   "Unable to determine the authenticated user",
			Retryable: false,
		})
	}
	if err != nil || resolved.Username != requester.Username || resolved.UID != requester.UID || resolved.GID != requester.GID {
		return capabilityUnavailable(message.MessageID)
	}
	drain, hasDrain := resolved.Capabilities[identity.CapabilityNodeDrain]
	resume, hasResume := resolved.Capabilities[identity.CapabilityNodeResume]
	if !hasDrain || !hasResume {
		return capabilityUnavailable(message.MessageID)
	}

	response := protocol.CapabilitiesResponsePayload{Capabilities: protocol.NodeCapabilities{
		NodesDrain:  drain,
		NodesResume: resume,
	}}
	payload, err := json.Marshal(response)
	if err != nil {
		return capabilityUnavailable(message.MessageID)
	}
	return protocol.NewEnvelope(message.MessageID, protocol.MessageCapabilitiesResponse, payload)
}

func capabilityUnavailable(messageID string) protocol.Envelope {
	return protocol.NewErrorEnvelope(messageID, protocol.ErrorPayload{
		Code:      protocol.ErrorCodeCapabilityUnavailable,
		Category:  protocol.ErrorCategoryInternal,
		Message:   "Capabilities are temporarily unavailable",
		Retryable: true,
	})
}
