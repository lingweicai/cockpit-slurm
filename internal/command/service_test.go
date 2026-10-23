package command

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/authorization"
	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/identity"
	"github.com/lingweicai/cockpit-slurm/internal/ledger"
	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
)

type authorizerStub struct {
	decision authorization.Decision
	err      error
	calls    int
}

func (a *authorizerStub) Authorize(_ context.Context, _ connection.UserContext, operation authorization.OperationID) (authorization.Decision, error) {
	a.calls++
	if a.err != nil {
		return authorization.Decision{}, a.err
	}
	decision := a.decision
	if decision.RequiredCapability == "" {
		switch operation {
		case authorization.OperationNodeDrain:
			decision.RequiredCapability = identity.CapabilityNodeDrain
		case authorization.OperationNodeResume:
			decision.RequiredCapability = identity.CapabilityNodeResume
		}
	}
	return decision, nil
}

type executorStub struct {
	identity  ExecutionIdentity
	calls     int
	operation authorization.OperationID
	node      string
	reason    string
	err       error
}

func (e *executorStub) Identity() ExecutionIdentity { return e.identity }

func (e *executorStub) Execute(_ context.Context, operation authorization.OperationID, node, reason string) error {
	e.calls++
	e.operation = operation
	e.node = node
	e.reason = reason
	return e.err
}

func TestHandleAuthorizedDrainPersistsAndDeduplicates(t *testing.T) {
	service, store, authorizer, executor, requester := newTestService(t, true)
	refreshRequested := make(chan struct{}, 1)
	service.SetNodeRefresh(func(context.Context) error {
		refreshRequested <- struct{}{}
		return nil
	})
	message := drainMessage("cmd-drain-1", "maintenance window")

	response := service.Handle(context.Background(), requester, message)
	if response.Type != protocol.MessageCommandResponse || response.MessageID != message.MessageID {
		t.Fatalf("response = %#v, want correlated command response", response)
	}
	var result protocol.NodeCommandResponsePayload
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatalf("decode command response: %v", err)
	}
	if result.Status != protocol.CommandStatusSucceeded || result.CommandID != "cmd-drain-1" {
		t.Fatalf("command result = %#v, want succeeded", result)
	}
	select {
	case <-refreshRequested:
	case <-time.After(time.Second):
		t.Fatal("successful command did not request Node synchronization")
	}
	if authorizer.calls != 1 || executor.calls != 1 || executor.operation != authorization.OperationNodeDrain || executor.node != "c21" || executor.reason != "maintenance window" {
		t.Fatalf("authorizer calls=%d executor=%#v; want one authorized drain of c21", authorizer.calls, executor)
	}

	record, found, err := store.Lookup(context.Background(), requester.UID, "cmd-drain-1")
	if err != nil || !found {
		t.Fatalf("ledger lookup found=%v err=%v, want record", found, err)
	}
	if record.Status != ledger.StatusSucceeded || record.AdminLevel != "admin" || record.ExecutionUID == nil || *record.ExecutionUID != executor.identity.UID || record.ExecutionUsername != executor.identity.Username || record.ExecutionBoundary != executor.identity.Boundary {
		t.Fatalf("persisted command record = %#v, want succeeded with separate requester/execution identity", record)
	}

	changedPermissionRetry := service.Handle(context.Background(), requester, drainMessage("cmd-drain-1", "maintenance window"))
	if changedPermissionRetry.Type != protocol.MessageCommandResponse || authorizer.calls != 1 || executor.calls != 1 {
		t.Fatalf("retry response=%#v authorizer calls=%d executor calls=%d; want stored result without reauthorization or re-execution", changedPermissionRetry, authorizer.calls, executor.calls)
	}
}

func TestDeniedCommandPersistsTombstoneWithoutExecution(t *testing.T) {
	service, store, authorizer, executor, requester := newTestService(t, false)
	message := drainMessage("cmd-denied-1", "maintenance")

	response := service.Handle(context.Background(), requester, message)
	var failure protocol.ErrorPayload
	if response.Type != protocol.MessageError || json.Unmarshal(response.Payload, &failure) != nil {
		t.Fatalf("response = %#v, want canonical authorization error", response)
	}
	if failure.Code != protocol.ErrorCodeAuthorizationDenied || failure.Category != protocol.ErrorCategoryAuthorization || failure.Retryable {
		t.Fatalf("authorization error = %#v", failure)
	}
	record, found, err := store.Lookup(context.Background(), requester.UID, "cmd-denied-1")
	if err != nil || !found || record.Status != ledger.StatusDenied {
		t.Fatalf("denied record = %#v found=%v err=%v, want durable denied tombstone", record, found, err)
	}
	if authorizer.calls != 1 || executor.calls != 0 {
		t.Fatalf("authorizer calls=%d executor calls=%d, want one authorization and no execution", authorizer.calls, executor.calls)
	}

	// A denied ID remains denied even if a later authorization decision would allow it.
	authorizer.decision.Allowed = true
	retry := service.Handle(context.Background(), requester, drainMessage("cmd-denied-1", "maintenance"))
	if retry.Type != protocol.MessageError || authorizer.calls != 1 || executor.calls != 0 {
		t.Fatalf("retry response=%#v authorizer calls=%d executor calls=%d; want stored denial without reauthorization", retry, authorizer.calls, executor.calls)
	}
}

func TestInvalidTargetDoesNotClaimOrAuthorize(t *testing.T) {
	service, store, authorizer, executor, requester := newTestService(t, true)
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("c21", 1)})
	service.nodes = cache
	message := drainMessage("cmd-missing-node", "maintenance")
	var request protocol.NodeCommandRequestPayload
	if err := json.Unmarshal(message.Payload, &request); err != nil {
		t.Fatal(err)
	}
	request.Target.NodeName = "missing"
	message.Payload, _ = json.Marshal(request)

	response := service.Handle(context.Background(), requester, message)
	if response.Type != protocol.MessageError || authorizer.calls != 0 || executor.calls != 0 {
		t.Fatalf("response=%#v authorizer calls=%d executor calls=%d; want validation failure before authorization/execution", response, authorizer.calls, executor.calls)
	}
	if _, found, err := store.Lookup(context.Background(), requester.UID, request.CommandID); err != nil || found {
		t.Fatalf("invalid request ledger record found=%v err=%v; want no claim", found, err)
	}
}

func TestMalformedNodeNameDoesNotClaimOrAuthorize(t *testing.T) {
	service, store, authorizer, executor, requester := newTestService(t, true)
	message := drainMessage("cmd-malformed-node", "maintenance")
	var request protocol.NodeCommandRequestPayload
	if err := json.Unmarshal(message.Payload, &request); err != nil {
		t.Fatal(err)
	}
	request.Target.NodeName = "c21,c22"
	message.Payload, _ = json.Marshal(request)

	response := service.Handle(context.Background(), requester, message)
	if response.Type != protocol.MessageError || authorizer.calls != 0 || executor.calls != 0 {
		t.Fatalf("response=%#v authorizer calls=%d executor calls=%d; want malformed target rejected before authorization/execution", response, authorizer.calls, executor.calls)
	}
	if _, found, err := store.Lookup(context.Background(), requester.UID, request.CommandID); err != nil || found {
		t.Fatalf("malformed request ledger record found=%v err=%v; want no claim", found, err)
	}
}

func TestAuthorizationLookupFailureIsDurablyNonExecutable(t *testing.T) {
	service, store, authorizer, executor, requester := newTestService(t, true)
	authorizer.err = errors.New("lookup failed")
	message := drainMessage("cmd-lookup-failure", "maintenance")

	response := service.Handle(context.Background(), requester, message)
	var failure protocol.ErrorPayload
	if response.Type != protocol.MessageError || json.Unmarshal(response.Payload, &failure) != nil || failure.Code != protocol.ErrorCodeUserLookupFailed {
		t.Fatalf("response=%#v, want user-lookup-failed", response)
	}
	record, found, err := store.Lookup(context.Background(), requester.UID, "cmd-lookup-failure")
	if err != nil || !found || record.Status != ledger.StatusAuthorizationFailed || executor.calls != 0 {
		t.Fatalf("record=%#v found=%v err=%v executor calls=%d; want authorization-failure tombstone and no execution", record, found, err, executor.calls)
	}
}

func newTestService(t *testing.T, allowed bool) (*Service, *ledger.Ledger, *authorizerStub, *executorStub, connection.UserContext) {
	t.Helper()
	store, err := ledger.Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("open test ledger: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	registry, err := authorization.NewNodeRegistry()
	if err != nil {
		t.Fatalf("create node registry: %v", err)
	}
	nodes := resource.NewNodeCache()
	nodes.ReplaceSnapshot([]resource.Node{resource.NewNode("c21", 1)})
	requester := connection.UserContext{UID: 1008, GID: 978, Username: "slurm_admin"}
	authorizer := &authorizerStub{
		decision: authorization.Decision{
			Allowed: allowed,
			UserContext: connection.UserContext{
				UID:        requester.UID,
				GID:        requester.GID,
				Username:   requester.Username,
				AdminLevel: "admin",
				Capabilities: map[string]bool{
					identity.CapabilityNodeDrain:  allowed,
					identity.CapabilityNodeResume: allowed,
				},
			},
		},
	}
	executor := &executorStub{identity: ExecutionIdentity{UID: 202, Username: "slurm", Boundary: "dbus-nodehelper"}}
	service, err := NewService(store, authorizer, executor, registry, nodes)
	if err != nil {
		t.Fatalf("create command service: %v", err)
	}
	return service, store, authorizer, executor, requester
}

func drainMessage(commandID, reason string) protocol.Envelope {
	payload, _ := json.Marshal(protocol.NodeCommandRequestPayload{
		CommandID: commandID,
		Resource:  "node",
		Command:   "update",
		Target:    protocol.NodeCommandTarget{NodeName: "c21"},
		Parameters: protocol.NodeCommandParameters{
			State:  "DRAIN",
			Reason: &reason,
		},
	})
	return protocol.NewEnvelope("message-"+commandID, protocol.MessageCommand, payload)
}
