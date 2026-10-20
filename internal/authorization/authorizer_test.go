package authorization

import (
	"context"
	"errors"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/identity"
)

type stubCapabilityResolver struct {
	results []connection.UserContext
	err     error
	calls   int
}

func (s *stubCapabilityResolver) Resolve(_ context.Context, _ connection.UserContext) (connection.UserContext, error) {
	s.calls++
	if s.err != nil {
		return connection.UserContext{}, s.err
	}
	index := s.calls - 1
	if index >= len(s.results) {
		index = len(s.results) - 1
	}
	return s.results[index], nil
}

func TestNodeRegistryHasExactOperationCapabilityMappings(t *testing.T) {
	registry, err := NewNodeRegistry()
	if err != nil {
		t.Fatalf("NewNodeRegistry() returned error: %v", err)
	}
	for _, test := range []struct {
		operation  OperationID
		capability string
	}{
		{OperationNodeDrain, identity.CapabilityNodeDrain},
		{OperationNodeResume, identity.CapabilityNodeResume},
	} {
		entry, ok := registry.Lookup(test.operation)
		if !ok {
			t.Fatalf("Lookup(%q) did not find operation", test.operation)
		}
		if entry.RequiredCapability != test.capability {
			t.Errorf("operation %q capability = %q, want %q", test.operation, entry.RequiredCapability, test.capability)
		}
	}
	if _, ok := registry.Lookup("job.cancel"); ok {
		t.Fatal("registry contains an out-of-scope operation")
	}
}

func TestGlobalRegistryRejectsInconsistentEntries(t *testing.T) {
	valid, err := NewNodeRegistry()
	if err != nil {
		t.Fatalf("NewNodeRegistry() returned error: %v", err)
	}
	base := map[OperationID]RegistryEntry{}
	for _, operation := range []OperationID{OperationNodeDrain, OperationNodeResume} {
		entry, _ := valid.Lookup(operation)
		base[operation] = entry
	}

	tests := []struct {
		name       string
		mutate     func(map[OperationID]RegistryEntry)
		handlers   map[string]struct{}
		validators map[string]struct{}
	}{
		{name: "key mismatch", mutate: func(entries map[OperationID]RegistryEntry) {
			entry := entries[OperationNodeDrain]
			entry.OperationID = OperationNodeResume
			entries[OperationNodeDrain] = entry
		}},
		{name: "wrong capability", mutate: func(entries map[OperationID]RegistryEntry) {
			entry := entries[OperationNodeDrain]
			entry.RequiredCapability = "nodes.resume"
			entries[OperationNodeDrain] = entry
		}},
		{name: "missing operation", mutate: func(entries map[OperationID]RegistryEntry) { delete(entries, OperationNodeResume) }},
		{name: "unknown handler", mutate: func(entries map[OperationID]RegistryEntry) {
			entry := entries[OperationNodeDrain]
			entry.HandlerID = "client.handler"
			entries[OperationNodeDrain] = entry
		}},
		{name: "unsupported parameter type", mutate: func(entries map[OperationID]RegistryEntry) {
			entry := entries[OperationNodeDrain]
			spec := entry.ParameterSchema["reason"]
			spec.Type = "shell"
			entry.ParameterSchema["reason"] = spec
			entries[OperationNodeDrain] = entry
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries := map[OperationID]RegistryEntry{}
			for operation, entry := range base {
				entries[operation] = cloneEntry(entry)
			}
			test.mutate(entries)
			handlers := test.handlers
			if handlers == nil {
				handlers = map[string]struct{}{NodeStateHandlerID: {}}
			}
			validators := test.validators
			if validators == nil {
				validators = map[string]struct{}{NodeTargetValidatorID: {}}
			}
			if _, err := NewGlobalRegistry(entries, handlers, validators); !errors.Is(err, ErrInvalidRegistry) {
				t.Fatalf("NewGlobalRegistry() error = %v, want %v", err, ErrInvalidRegistry)
			}
		})
	}
}

func TestAuthorizerUsesFreshCapabilitiesAndNeverFallsBackToSnapshot(t *testing.T) {
	registry, err := NewNodeRegistry()
	if err != nil {
		t.Fatalf("NewNodeRegistry() returned error: %v", err)
	}
	resolver := &stubCapabilityResolver{results: []connection.UserContext{
		{UID: 1008, GID: 978, Username: "slurm_admin", AdminLevel: "admin", Capabilities: map[string]bool{identity.CapabilityNodeDrain: true, identity.CapabilityNodeResume: true}},
		{UID: 1008, GID: 978, Username: "slurm_admin", AdminLevel: "operator", Capabilities: map[string]bool{identity.CapabilityNodeDrain: false, identity.CapabilityNodeResume: false}},
	}}
	authorizer := NewAuthorizer(registry, resolver)
	requester := connection.UserContext{
		UID:          1008,
		GID:          978,
		Username:     "slurm_admin",
		AdminLevel:   "admin",
		Capabilities: map[string]bool{identity.CapabilityNodeDrain: true, identity.CapabilityNodeResume: true},
	}

	first, err := authorizer.Authorize(context.Background(), requester, OperationNodeDrain)
	if err != nil || !first.Allowed {
		t.Fatalf("first authorization = %#v, err = %v; want allowed", first, err)
	}
	second, err := authorizer.Authorize(context.Background(), requester, OperationNodeDrain)
	if err != nil || second.Allowed {
		t.Fatalf("second authorization = %#v, err = %v; want denied after fresh permission change", second, err)
	}
	if resolver.calls != 2 {
		t.Fatalf("resolver calls = %d, want fresh resolution for each authorization", resolver.calls)
	}
}

func TestAuthorizerFailsClosedOnUnknownOperationAndResolverFailure(t *testing.T) {
	registry, err := NewNodeRegistry()
	if err != nil {
		t.Fatalf("NewNodeRegistry() returned error: %v", err)
	}
	resolver := &stubCapabilityResolver{err: identity.ErrSlurmUserNotFound}
	authorizer := NewAuthorizer(registry, resolver)
	requester := connection.UserContext{Username: "slurm_none"}

	if _, err := authorizer.Authorize(context.Background(), requester, "node.power"); !errors.Is(err, ErrUnknownOperation) {
		t.Fatalf("unknown operation error = %v, want %v", err, ErrUnknownOperation)
	}
	if resolver.calls != 0 {
		t.Fatalf("resolver called %d times for unknown operation, want 0", resolver.calls)
	}
	if _, err := authorizer.Authorize(context.Background(), requester, OperationNodeDrain); !errors.Is(err, identity.ErrSlurmUserNotFound) {
		t.Fatalf("resolver failure = %v, want %v", err, identity.ErrSlurmUserNotFound)
	}
}

func TestAuthorizerRejectsResolverIdentityMismatch(t *testing.T) {
	registry, err := NewNodeRegistry()
	if err != nil {
		t.Fatalf("NewNodeRegistry() returned error: %v", err)
	}
	resolver := &stubCapabilityResolver{results: []connection.UserContext{{
		UID:          0,
		GID:          0,
		Username:     "root",
		Capabilities: map[string]bool{identity.CapabilityNodeDrain: true},
	}}}
	authorizer := NewAuthorizer(registry, resolver)
	requester := connection.UserContext{UID: 1008, GID: 978, Username: "slurm_admin"}
	if _, err := authorizer.Authorize(context.Background(), requester, OperationNodeDrain); !errors.Is(err, identity.ErrIdentityUnavailable) {
		t.Fatalf("Authorize() error = %v, want %v", err, identity.ErrIdentityUnavailable)
	}
}
