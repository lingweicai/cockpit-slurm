package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/identity"
	"github.com/lingweicai/cockpit-slurm/internal/protocol"
)

type capabilityResolverStub struct {
	resolved  connection.UserContext
	err       error
	requester connection.UserContext
	calls     int
}

func (s *capabilityResolverStub) Resolve(_ context.Context, requester connection.UserContext) (connection.UserContext, error) {
	s.calls++
	s.requester = requester
	return s.resolved, s.err
}

func TestCapabilitiesHandlerReturnsCorrelatedExplicitCapabilities(t *testing.T) {
	resolver := &capabilityResolverStub{resolved: connection.UserContext{
		UID:          1008,
		GID:          978,
		Username:     "slurm_admin",
		AdminLevel:   "admin",
		Capabilities: map[string]bool{identity.CapabilityNodeDrain: true, identity.CapabilityNodeResume: true},
	}}
	handler := NewCapabilitiesHandler(resolver)
	requester := connection.UserContext{UID: 1008, GID: 978, Username: "slurm_admin"}
	request := protocol.NewEnvelope("cap-request-1", protocol.MessageCapabilitiesRequest, json.RawMessage(`{}`))

	response := handler.HandleAuthenticated(context.Background(), requester, request)
	if response.Type != protocol.MessageCapabilitiesResponse || response.MessageID != request.MessageID {
		t.Fatalf("response envelope = %#v, want correlated capabilities-response", response)
	}
	capabilities, err := protocol.DecodeCapabilitiesResponsePayload(response.Payload)
	if err != nil {
		t.Fatalf("decode capabilities response: %v", err)
	}
	if !capabilities.Capabilities.NodesDrain || !capabilities.Capabilities.NodesResume {
		t.Fatalf("capabilities = %#v, want both true", capabilities.Capabilities)
	}
	if resolver.calls != 1 || resolver.requester.UID != requester.UID || resolver.requester.GID != requester.GID || resolver.requester.Username != requester.Username {
		t.Fatalf("resolver calls=%d requester=%#v, want one call with server identity %#v", resolver.calls, resolver.requester, requester)
	}
}

func TestCapabilitiesHandlerReturnsExplicitFalseForNonAdmin(t *testing.T) {
	resolver := &capabilityResolverStub{resolved: connection.UserContext{
		UID:          1007,
		GID:          978,
		Username:     "slurm_operator",
		AdminLevel:   "operator",
		Capabilities: map[string]bool{identity.CapabilityNodeDrain: false, identity.CapabilityNodeResume: false},
	}}
	handler := NewCapabilitiesHandler(resolver)
	requester := connection.UserContext{UID: 1007, GID: 978, Username: "slurm_operator"}
	request := protocol.NewEnvelope("cap-request-operator", protocol.MessageCapabilitiesRequest, json.RawMessage(`{}`))

	response := handler.HandleAuthenticated(context.Background(), requester, request)
	capabilities, err := protocol.DecodeCapabilitiesResponsePayload(response.Payload)
	if err != nil {
		t.Fatalf("decode operator capabilities: %v", err)
	}
	if capabilities.Capabilities.NodesDrain || capabilities.Capabilities.NodesResume {
		t.Fatalf("operator capabilities = %#v, want explicit false values", capabilities.Capabilities)
	}
}

func TestCapabilitiesHandlerReturnsCanonicalErrors(t *testing.T) {
	request := protocol.NewEnvelope("cap-request-2", protocol.MessageCapabilitiesRequest, json.RawMessage(`{}`))
	requester := connection.UserContext{UID: 1006, GID: 978, Username: "slurm_none"}

	t.Run("lookup failure", func(t *testing.T) {
		handler := NewCapabilitiesHandler(&capabilityResolverStub{err: identity.ErrSlurmUserNotFound})
		response := handler.HandleAuthenticated(context.Background(), requester, request)
		assertCapabilitiesError(t, response, request.MessageID, "capability-unavailable", protocol.ErrorCategoryInternal, true)
	})

	t.Run("identity resolution failure", func(t *testing.T) {
		handler := NewCapabilitiesHandler(&capabilityResolverStub{err: identity.ErrIdentityUnavailable})
		response := handler.HandleAuthenticated(context.Background(), requester, request)
		assertCapabilitiesError(t, response, request.MessageID, "identity-unavailable", protocol.ErrorCategoryAuthentication, false)
	})

	t.Run("invalid request", func(t *testing.T) {
		resolver := &capabilityResolverStub{}
		handler := NewCapabilitiesHandler(resolver)
		invalid := protocol.NewEnvelope("cap-request-3", protocol.MessageCapabilitiesRequest, json.RawMessage(`{"extra":true}`))
		response := handler.HandleAuthenticated(context.Background(), requester, invalid)
		assertCapabilitiesError(t, response, invalid.MessageID, "invalid-payload", protocol.ErrorCategoryValidation, false)
		if resolver.calls != 0 {
			t.Fatalf("resolver called %d times for invalid payload, want 0", resolver.calls)
		}
	})

	t.Run("resolver changed identity", func(t *testing.T) {
		resolver := &capabilityResolverStub{resolved: connection.UserContext{
			UID:          0,
			GID:          0,
			Username:     "root",
			Capabilities: map[string]bool{identity.CapabilityNodeDrain: true, identity.CapabilityNodeResume: true},
		}}
		handler := NewCapabilitiesHandler(resolver)
		response := handler.HandleAuthenticated(context.Background(), requester, request)
		assertCapabilitiesError(t, response, request.MessageID, "capability-unavailable", protocol.ErrorCategoryInternal, true)
	})
}

func assertCapabilitiesError(t *testing.T, response protocol.Envelope, messageID string, code protocol.ErrorCode, category protocol.ErrorCategory, retryable bool) {
	t.Helper()
	if response.Type != protocol.MessageError || response.MessageID != messageID {
		t.Fatalf("error envelope = %#v, want correlated error", response)
	}
	var payload protocol.ErrorPayload
	if err := json.Unmarshal(response.Payload, &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != code || payload.Category != category || payload.Retryable != retryable || payload.Message == "" {
		t.Fatalf("error payload = %#v, want code=%q category=%q retryable=%t and message", payload, code, category, retryable)
	}
}
