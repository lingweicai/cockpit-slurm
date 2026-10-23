package command

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
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

type blockingAuthorizer struct {
	mu       sync.Mutex
	calls    int
	entered  chan struct{}
	release  chan struct{}
	decision authorization.Decision
}

func (a *blockingAuthorizer) Authorize(_ context.Context, _ connection.UserContext, operation authorization.OperationID) (authorization.Decision, error) {
	a.mu.Lock()
	a.calls++
	a.mu.Unlock()
	a.entered <- struct{}{}
	<-a.release
	decision := a.decision
	if decision.RequiredCapability == "" {
		if operation == authorization.OperationNodeDrain {
			decision.RequiredCapability = identity.CapabilityNodeDrain
		} else {
			decision.RequiredCapability = identity.CapabilityNodeResume
		}
	}
	return decision, nil
}

type observedLedger struct {
	Ledger
	lookedUp chan bool
}

type transitionFaultLedger struct {
	Ledger
	mu           sync.Mutex
	blocked      bool
	callerUID    uint32
	commandID    string
	reconcileErr error
	expectedFrom ledger.Status
	expectedTo   ledger.Status
	commitFirst  bool
}

func (l *transitionFaultLedger) AdmissionBlocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.blocked || l.Ledger.AdmissionBlocked()
}

func (l *transitionFaultLedger) Transition(ctx context.Context, uid uint32, commandID string, expected, next ledger.Status, data ledger.TransitionData) error {
	if expected == l.expectedFrom && next == l.expectedTo {
		if l.commitFirst {
			if err := l.Ledger.Transition(ctx, uid, commandID, expected, next, data); err != nil {
				return err
			}
		}
		l.mu.Lock()
		l.blocked = true
		l.mu.Unlock()
		return errors.New("injected uncertain transition result")
	}
	return l.Ledger.Transition(ctx, uid, commandID, expected, next, data)
}

func (l *transitionFaultLedger) Reconcile(ctx context.Context) error {
	if l.reconcileErr != nil {
		return l.reconcileErr
	}
	record, found, err := l.Ledger.Lookup(ctx, l.callerUID, l.commandID)
	if err != nil {
		return err
	}
	if found && (record.Status == ledger.StatusAuthorizationPending || record.Status == ledger.StatusAccepted) {
		if err := l.Ledger.Transition(ctx, l.callerUID, l.commandID, record.Status, ledger.StatusAuthorizationUncertain, ledger.TransitionData{}); err != nil {
			return err
		}
	}
	l.mu.Lock()
	l.blocked = false
	l.mu.Unlock()
	return nil
}

func (l observedLedger) Lookup(ctx context.Context, callerUID uint32, commandID string) (ledger.Record, bool, error) {
	record, found, err := l.Ledger.Lookup(ctx, callerUID, commandID)
	l.lookedUp <- found && err == nil
	return record, found, err
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
	if record.Status != ledger.StatusSucceeded || record.AdminLevel != "admin" || record.RequestingGID != requester.GID || record.RequestMessageID != message.MessageID || record.ServiceUID != service.serviceUID || record.ServiceUsername != service.serviceUser || record.ExecutionUID == nil || *record.ExecutionUID != executor.identity.UID || record.ExecutionUsername != executor.identity.Username || record.ExecutionBoundary != executor.identity.Boundary {
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

func TestHandleAuthorizedResumeUsesFixedOperation(t *testing.T) {
	service, store, authorizer, executor, requester := newTestService(t, true)
	message := resumeMessage("cmd-resume-1")
	response := service.Handle(context.Background(), requester, message)
	if response.Type != protocol.MessageCommandResponse {
		t.Fatalf("response = %#v, want command response", response)
	}
	if authorizer.calls != 1 || executor.calls != 1 || executor.operation != authorization.OperationNodeResume || executor.node != "c21" || executor.reason != "" {
		t.Fatalf("authorizer calls=%d executor=%#v; want one fixed resume operation", authorizer.calls, executor)
	}
	record, found, err := store.Lookup(context.Background(), requester.UID, "cmd-resume-1")
	if err != nil || !found || record.Status != ledger.StatusSucceeded || record.OperationID != string(authorization.OperationNodeResume) {
		t.Fatalf("resume record=%#v found=%v err=%v", record, found, err)
	}
}

func TestExecutionFailureAndUnknownOutcomeArePersistedSeparately(t *testing.T) {
	for _, test := range []struct {
		name          string
		commandID     string
		executionErr  error
		wantStatus    ledger.Status
		wantErrorCode protocol.ErrorCode
	}{
		{name: "definite failure", commandID: "cmd-definite-failure", executionErr: errors.New("helper reports Slurm failure"), wantStatus: ledger.StatusFailed, wantErrorCode: protocol.ErrorCodeSlurmCommandFailed},
		{name: "unknown outcome", commandID: "cmd-unknown-outcome", executionErr: ErrExecutionOutcomeUnknown, wantStatus: ledger.StatusUnknown, wantErrorCode: protocol.ErrorCodeCommandOutcomeUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, store, _, executor, requester := newTestService(t, true)
			executor.err = test.executionErr
			refreshRequested := make(chan struct{}, 1)
			service.SetNodeRefresh(func(context.Context) error {
				refreshRequested <- struct{}{}
				return nil
			})
			message := resumeMessage(test.commandID)
			response := service.Handle(context.Background(), requester, message)
			var failure protocol.ErrorPayload
			if response.Type != protocol.MessageError || json.Unmarshal(response.Payload, &failure) != nil || failure.Code != test.wantErrorCode {
				t.Fatalf("response=%#v failure=%#v; want error %s", response, failure, test.wantErrorCode)
			}
			record, found, err := store.Lookup(context.Background(), requester.UID, test.commandID)
			if err != nil || !found || record.Status != test.wantStatus || executor.calls != 1 {
				t.Fatalf("record=%#v found=%v err=%v executor calls=%d; want %s", record, found, err, executor.calls, test.wantStatus)
			}
			if test.wantStatus == ledger.StatusUnknown {
				select {
				case <-refreshRequested:
				case <-time.After(time.Second):
					t.Fatal("unknown command outcome did not request Node synchronization")
				}
			} else {
				select {
				case <-refreshRequested:
					t.Fatal("definite command failure unexpectedly requested Node synchronization")
				default:
				}
			}
		})
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

func TestAuthorizationIdentityMismatchNeverExecutes(t *testing.T) {
	service, store, authorizer, executor, requester := newTestService(t, true)
	authorizer.decision.UserContext.UID++
	message := drainMessage("cmd-identity-mismatch", "maintenance")

	response := service.Handle(context.Background(), requester, message)
	var failure protocol.ErrorPayload
	if response.Type != protocol.MessageError || json.Unmarshal(response.Payload, &failure) != nil || failure.Code != protocol.ErrorCodeInternal {
		t.Fatalf("response=%#v, want fail-closed internal authorization error", response)
	}
	record, found, err := store.Lookup(context.Background(), requester.UID, "cmd-identity-mismatch")
	if err != nil || !found || record.Status != ledger.StatusAuthorizationFailed || authorizer.calls != 1 || executor.calls != 0 {
		t.Fatalf("record=%#v found=%v err=%v authorizer calls=%d executor calls=%d; want authorization-failure tombstone and no execution", record, found, err, authorizer.calls, executor.calls)
	}
}

func TestConcurrentServicesWaitForSingleCommandOwner(t *testing.T) {
	firstService, store, _, executor, requester := newTestService(t, true)
	blocking := &blockingAuthorizer{
		entered: make(chan struct{}, 2),
		release: make(chan struct{}),
		decision: authorization.Decision{
			Allowed: true,
			UserContext: connection.UserContext{
				UID: requester.UID, GID: requester.GID, Username: requester.Username, AdminLevel: "admin",
				Capabilities: map[string]bool{identity.CapabilityNodeDrain: true},
			},
		},
	}
	firstService.authorizer = blocking
	secondService, err := NewService(observedLedger{Ledger: store, lookedUp: make(chan bool, 1)}, blocking, executor, firstService.registry, firstService.nodes)
	if err != nil {
		t.Fatalf("create second command service: %v", err)
	}
	ledgerObserver := secondService.ledger.(observedLedger)
	message := drainMessage("cmd-concurrent-1", "maintenance")
	firstResponse := make(chan protocol.Envelope, 1)
	secondResponse := make(chan protocol.Envelope, 1)
	go func() { firstResponse <- firstService.Handle(context.Background(), requester, message) }()
	<-blocking.entered
	go func() {
		secondResponse <- secondService.Handle(context.Background(), requester, drainMessage("cmd-concurrent-1", "maintenance"))
	}()
	if found := <-ledgerObserver.lookedUp; !found {
		t.Fatal("second request did not observe the existing command claim")
	}

	close(blocking.release)
	responses := []protocol.Envelope{<-firstResponse, <-secondResponse}
	for _, response := range responses {
		if response.Type != protocol.MessageCommandResponse {
			t.Fatalf("concurrent response = %#v, want command response", response)
		}
	}
	blocking.mu.Lock()
	authorizerCalls := blocking.calls
	blocking.mu.Unlock()
	if authorizerCalls != 1 || executor.calls != 1 {
		t.Fatalf("authorizer calls=%d executor calls=%d, want exactly one each", authorizerCalls, executor.calls)
	}
}

func TestConcurrentConflictingFingerprintReturnsCommandIDConflict(t *testing.T) {
	firstService, store, _, executor, requester := newTestService(t, true)
	blocking := &blockingAuthorizer{
		entered: make(chan struct{}, 2),
		release: make(chan struct{}),
		decision: authorization.Decision{
			Allowed:     true,
			UserContext: connection.UserContext{UID: requester.UID, GID: requester.GID, Username: requester.Username, AdminLevel: "admin"},
		},
	}
	firstService.authorizer = blocking
	secondService, err := NewService(store, blocking, executor, firstService.registry, firstService.nodes)
	if err != nil {
		t.Fatalf("create second command service: %v", err)
	}
	firstResponse := make(chan protocol.Envelope, 1)
	go func() {
		firstResponse <- firstService.Handle(context.Background(), requester, drainMessage("cmd-conflict-race", "maintenance"))
	}()
	<-blocking.entered
	secondMessage := drainMessage("cmd-conflict-race", "different maintenance")
	secondResponse := secondService.Handle(context.Background(), requester, secondMessage)
	var failure protocol.ErrorPayload
	if secondResponse.Type != protocol.MessageError || json.Unmarshal(secondResponse.Payload, &failure) != nil || failure.Code != protocol.ErrorCodeCommandIDConflict {
		t.Fatalf("conflicting response=%#v, want command-id-conflict", secondResponse)
	}
	close(blocking.release)
	if response := <-firstResponse; response.Type != protocol.MessageCommandResponse {
		t.Fatalf("owner response = %#v, want command response", response)
	}
	blocking.mu.Lock()
	authorizerCalls := blocking.calls
	blocking.mu.Unlock()
	if authorizerCalls != 1 || executor.calls != 1 {
		t.Fatalf("authorizer calls=%d executor calls=%d, want exactly one each", authorizerCalls, executor.calls)
	}
}

func TestUncertainAuthorizationTransitionNeverDispatches(t *testing.T) {
	for _, test := range []struct {
		name        string
		commitFirst bool
	}{
		{name: "commit before error", commitFirst: true},
		{name: "pre-commit error", commitFirst: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, store, authorizer, executor, requester := newTestService(t, true)
			faultLedger := &transitionFaultLedger{
				Ledger: store, expectedFrom: ledger.StatusAuthorizationPending,
				expectedTo: ledger.StatusAccepted, commitFirst: test.commitFirst,
				callerUID: requester.UID, commandID: "cmd-transition-fault",
			}
			service.ledger = faultLedger
			message := drainMessage("cmd-transition-fault", "maintenance")
			response := service.Handle(context.Background(), requester, message)
			var failure protocol.ErrorPayload
			if response.Type != protocol.MessageError || json.Unmarshal(response.Payload, &failure) != nil || failure.Code != protocol.ErrorCodeLedgerPersistenceFailed {
				t.Fatalf("response=%#v failure=%#v; want ledger-persistence-failed", response, failure)
			}
			if authorizer.calls != 1 || executor.calls != 0 || faultLedger.AdmissionBlocked() {
				t.Fatalf("authorizer calls=%d executor calls=%d admission blocked=%v", authorizer.calls, executor.calls, faultLedger.AdmissionBlocked())
			}
			record, found, err := store.Lookup(context.Background(), requester.UID, "cmd-transition-fault")
			if err != nil || !found || record.Status != ledger.StatusAuthorizationUncertain {
				t.Fatalf("record=%#v found=%v err=%v; want quarantined authorization_uncertain", record, found, err)
			}
		})
	}

	t.Run("reconciliation failure leaves admission blocked", func(t *testing.T) {
		service, store, authorizer, executor, requester := newTestService(t, true)
		faultLedger := &transitionFaultLedger{
			Ledger: store, expectedFrom: ledger.StatusAuthorizationPending,
			expectedTo: ledger.StatusAccepted, callerUID: requester.UID,
			commandID: "cmd-reconcile-failure", reconcileErr: errors.New("integrity check failed"),
		}
		service.ledger = faultLedger
		response := service.Handle(context.Background(), requester, drainMessage("cmd-reconcile-failure", "maintenance"))
		if response.Type != protocol.MessageError || !faultLedger.AdmissionBlocked() || executor.calls != 0 || authorizer.calls != 1 {
			t.Fatalf("response=%#v admission blocked=%v authorizer calls=%d executor calls=%d", response, faultLedger.AdmissionBlocked(), authorizer.calls, executor.calls)
		}
	})
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

func resumeMessage(commandID string) protocol.Envelope {
	payload, _ := json.Marshal(protocol.NodeCommandRequestPayload{
		CommandID:  commandID,
		Resource:   "node",
		Command:    "update",
		Target:     protocol.NodeCommandTarget{NodeName: "c21"},
		Parameters: protocol.NodeCommandParameters{State: "RESUME"},
	})
	return protocol.NewEnvelope("message-"+commandID, protocol.MessageCommand, payload)
}
