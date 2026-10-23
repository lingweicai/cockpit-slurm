package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/authorization"
	"github.com/lingweicai/cockpit-slurm/internal/command"
	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/dispatcher"
	"github.com/lingweicai/cockpit-slurm/internal/identity"
	"github.com/lingweicai/cockpit-slurm/internal/ledger"
	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
	"github.com/lingweicai/cockpit-slurm/internal/stream"
)

type mutableNodeAdapter struct {
	mu    sync.RWMutex
	nodes []resource.Node
}

type transportCommandHandler struct{}

func (transportCommandHandler) Handle(_ context.Context, message protocol.Envelope) protocol.Envelope {
	return protocol.NewEnvelope(message.MessageID, protocol.MessageCommandResponse, json.RawMessage(`{"result":"transport-test"}`))
}

type countingCommandHandler struct {
	mu    sync.Mutex
	calls int
}

type adminCapabilityResolver struct {
	mu    sync.Mutex
	calls int
}

type nonAdminCapabilityResolver struct {
	mu    sync.Mutex
	calls int
}

type changeableCapabilityResolver struct {
	mu    sync.Mutex
	admin bool
	calls int
}

func (r *changeableCapabilityResolver) Resolve(_ context.Context, requester connection.UserContext) (connection.UserContext, error) {
	r.mu.Lock()
	admin := r.admin
	r.calls++
	r.mu.Unlock()
	requester.AdminLevel = "operator"
	if admin {
		requester.AdminLevel = "admin"
	}
	requester.Capabilities = map[string]bool{
		identity.CapabilityNodeDrain:  admin,
		identity.CapabilityNodeResume: admin,
	}
	return requester, nil
}

func (r *nonAdminCapabilityResolver) Resolve(_ context.Context, requester connection.UserContext) (connection.UserContext, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	requester.AdminLevel = "operator"
	requester.Capabilities = map[string]bool{
		identity.CapabilityNodeDrain:  false,
		identity.CapabilityNodeResume: false,
	}
	return requester, nil
}

func (r *adminCapabilityResolver) Resolve(_ context.Context, requester connection.UserContext) (connection.UserContext, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	requester.AdminLevel = "admin"
	requester.Capabilities = map[string]bool{
		identity.CapabilityNodeDrain:  true,
		identity.CapabilityNodeResume: true,
	}
	return requester, nil
}

type ipcNodeExecutor struct {
	mu        sync.Mutex
	calls     int
	operation authorization.OperationID
	node      string
	reason    string
	onExecute func(authorization.OperationID, string, string)
}

type ipcTransitionFaultLedger struct {
	command.Ledger
	mu      sync.Mutex
	blocked bool
}

func (l *ipcTransitionFaultLedger) AdmissionBlocked() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.blocked || l.Ledger.AdmissionBlocked()
}

func (l *ipcTransitionFaultLedger) Transition(ctx context.Context, uid uint32, commandID string, expected, next ledger.Status, data ledger.TransitionData) error {
	if expected == ledger.StatusAuthorizationPending && next == ledger.StatusAccepted {
		l.mu.Lock()
		l.blocked = true
		l.mu.Unlock()
		return errors.New("injected uncertain ledger transition")
	}
	return l.Ledger.Transition(ctx, uid, commandID, expected, next, data)
}

func (l *ipcTransitionFaultLedger) Reconcile(context.Context) error {
	return errors.New("injected ledger integrity failure")
}

func (e *ipcNodeExecutor) Identity() command.ExecutionIdentity {
	return command.ExecutionIdentity{UID: 202, Username: "slurm", Boundary: "test-executor"}
}

func (e *ipcNodeExecutor) Execute(_ context.Context, operation authorization.OperationID, nodeName, reason string) error {
	e.mu.Lock()
	e.calls++
	e.operation = operation
	e.node = nodeName
	e.reason = reason
	onExecute := e.onExecute
	e.mu.Unlock()
	if onExecute != nil {
		onExecute(operation, nodeName, reason)
	}
	return nil
}

func (h *countingCommandHandler) Handle(_ context.Context, message protocol.Envelope) protocol.Envelope {
	h.mu.Lock()
	h.calls++
	calls := h.calls
	h.mu.Unlock()
	payload, _ := json.Marshal(map[string]int{"calls": calls})
	return protocol.NewEnvelope(message.MessageID, protocol.MessageCommandResponse, payload)
}

type capabilityResolverForIPC struct {
	requester chan connection.UserContext
}

func (r *capabilityResolverForIPC) Resolve(_ context.Context, requester connection.UserContext) (connection.UserContext, error) {
	r.requester <- requester
	requester.Capabilities = map[string]bool{"nodes.drain": true, "nodes.resume": true}
	return requester, nil
}

func (a *mutableNodeAdapter) ListNodes(context.Context) ([]resource.Node, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return append([]resource.Node(nil), a.nodes...), nil
}

func (a *mutableNodeAdapter) setNodes(nodes ...resource.Node) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nodes = append([]resource.Node(nil), nodes...)
}

func TestServerStreamsNodeSnapshotAndCleansUpDisconnectedClient(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 0)})
	registry, err := stream.NewNodeStreamRegistry(cache, stream.DefaultNodeStreamQueueSize)
	if err != nil {
		t.Fatalf("NewNodeStreamRegistry returned error: %v", err)
	}

	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithNodeStreams(socketPath, registry)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ctx)
	}()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	decoder := protocol.NewDecoder(client)
	envelope, err := decoder.Decode()
	if err != nil {
		t.Fatalf("decode initial stream message: %v", err)
	}
	if envelope.Type != protocol.MessageEvent {
		t.Fatalf("message type = %q, want event", envelope.Type)
	}
	var payload struct {
		Resource   string          `json:"resource"`
		Event      string          `json:"event"`
		Generation uint64          `json:"generation"`
		Nodes      []resource.Node `json:"nodes"`
	}
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("decode initial payload: %v", err)
	}
	if payload.Resource != "node" || payload.Event != "snapshot" || payload.Generation != 1 {
		t.Fatalf("initial payload = %#v, want Node snapshot at generation 1", payload)
	}
	if len(payload.Nodes) != 1 || payload.Nodes[0].Identity() != "node001" {
		t.Fatalf("snapshot nodes = %#v, want node001", payload.Nodes)
	}
	if got := server.connections.Count(); got != 1 {
		t.Fatalf("registered connections = %d, want 1", got)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	waitForStreamCleanup(t, registry)

	if err := server.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve returned error after close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not exit after close")
	}
}

func TestNodeStreamConnectionDispatchesWhilePublishingEvents(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 0)})
	registry, err := stream.NewNodeStreamRegistry(cache, stream.DefaultNodeStreamQueueSize)
	if err != nil {
		t.Fatalf("NewNodeStreamRegistry returned error: %v", err)
	}

	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithNodeStreams(socketPath, registry)
	if err := server.dispatcher.Register(protocol.MessageCommand, transportCommandHandler{}); err != nil {
		t.Fatalf("register test command handler: %v", err)
	}
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(ctx) }()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	decoder := protocol.NewDecoder(client)
	snapshot, err := decoder.Decode()
	if err != nil {
		t.Fatalf("decode initial snapshot: %v", err)
	}
	if snapshot.Type != protocol.MessageEvent {
		t.Fatalf("initial message type = %q, want event", snapshot.Type)
	}

	request := protocol.NewEnvelope("COMMAND-DURING-STREAM", protocol.MessageCommand, json.RawMessage(`{"test":"transport"}`))
	writeDone := make(chan error, 1)
	go func() { writeDone <- protocol.NewEncoder(client).Encode(request) }()

	updated := resource.NewNode("node001", 0)
	updated.Status.State = "DOWN"
	cache.ReplaceSnapshotAndPublish([]resource.Node{updated}, registry)
	if err := <-writeDone; err != nil {
		t.Fatalf("send command while publishing: %v", err)
	}

	gotCommandResponse := false
	gotChanges := false
	for range 2 {
		message, err := decoder.Decode()
		if err != nil {
			t.Fatalf("decode concurrent server message: %v", err)
		}
		switch message.Type {
		case protocol.MessageCommandResponse:
			if message.MessageID != request.MessageID {
				t.Fatalf("command response message ID = %q, want %q", message.MessageID, request.MessageID)
			}
			gotCommandResponse = true
		case protocol.MessageEvent:
			var payload struct {
				Event      string `json:"event"`
				Generation uint64 `json:"generation"`
			}
			if err := json.Unmarshal(message.Payload, &payload); err != nil {
				t.Fatalf("decode change event payload: %v", err)
			}
			if payload.Event != "changes" || payload.Generation != 2 {
				t.Fatalf("change event = %#v, want generation 2 changes", payload)
			}
			gotChanges = true
		default:
			t.Fatalf("unexpected server message type %q", message.Type)
		}
	}
	if !gotCommandResponse || !gotChanges {
		t.Fatalf("received command response=%t changes=%t, want both", gotCommandResponse, gotChanges)
	}
}

func TestIPCConnectionReplaysMessageResponseAndRejectsMessageIDConflict(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServer(socketPath)
	handler := &countingCommandHandler{}
	if err := server.dispatcher.Register(protocol.MessageCommand, handler); err != nil {
		t.Fatalf("register command handler: %v", err)
	}
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = server.Serve(ctx) }()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	encoder := protocol.NewEncoder(client)
	decoder := protocol.NewDecoder(client)
	request := protocol.NewEnvelope("REPLAY-1", protocol.MessageCommand, json.RawMessage(`{"operation":"drain","node":"c21"}`))
	if err := encoder.Encode(request); err != nil {
		t.Fatalf("send initial command: %v", err)
	}
	initialResponse, err := decoder.Decode()
	if err != nil {
		t.Fatalf("read initial command response: %v", err)
	}

	replay := request
	replay.Timestamp = "later"
	replay.Payload = json.RawMessage(`{ "node" : "c21", "operation" : "drain" }`)
	if err := encoder.Encode(replay); err != nil {
		t.Fatalf("send replay: %v", err)
	}
	replayResponse, err := decoder.Decode()
	if err != nil {
		t.Fatalf("read replay response: %v", err)
	}
	if replayResponse.Type != initialResponse.Type || string(replayResponse.Payload) != string(initialResponse.Payload) {
		t.Fatalf("replay response=%#v, initial response=%#v; want identical cached response", replayResponse, initialResponse)
	}

	conflict := request
	conflict.Payload = json.RawMessage(`{"operation":"resume","node":"c21"}`)
	if err := encoder.Encode(conflict); err != nil {
		t.Fatalf("send conflicting message ID: %v", err)
	}
	conflictResponse, err := decoder.Decode()
	if err != nil {
		t.Fatalf("read message ID conflict response: %v", err)
	}
	var failure protocol.ErrorPayload
	if conflictResponse.Type != protocol.MessageError || json.Unmarshal(conflictResponse.Payload, &failure) != nil || failure.Code != protocol.ErrorCodeMessageIDConflict {
		t.Fatalf("conflict response=%#v failure=%#v, want message-id-conflict", conflictResponse, failure)
	}
	handler.mu.Lock()
	calls := handler.calls
	handler.mu.Unlock()
	if calls != 1 {
		t.Fatalf("command handler calls = %d, want 1", calls)
	}
}

func TestAuthenticatedCommandRunsThroughIPCServiceAndLedger(t *testing.T) {
	cache := resource.NewNodeCache()
	initialNode := resource.NewNode("c21", 0)
	initialNode.Status.State = "IDLE"
	cache.ReplaceSnapshot([]resource.Node{initialNode})
	adapter := &mutableNodeAdapter{nodes: []resource.Node{initialNode}}
	streams, err := stream.NewNodeStreamRegistry(cache, stream.DefaultNodeStreamQueueSize)
	if err != nil {
		t.Fatalf("create Node stream registry: %v", err)
	}
	synchronizer, err := resource.NewNodeSynchronizer(adapter, cache, streams, time.Hour)
	if err != nil {
		t.Fatalf("create Node synchronizer: %v", err)
	}
	registry, err := authorization.NewNodeRegistry()
	if err != nil {
		t.Fatalf("create Node registry: %v", err)
	}
	store, err := ledger.Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("open test ledger: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	resolver := &adminCapabilityResolver{}
	authorizer := authorization.NewAuthorizer(registry, resolver)
	executor := &ipcNodeExecutor{onExecute: func(operation authorization.OperationID, nodeName, reason string) {
		if operation != authorization.OperationNodeDrain || nodeName != "c21" || reason != "scheduled maintenance" {
			return
		}
		updatedNode := resource.NewNode(nodeName, 0)
		updatedNode.Status.State = "DRAIN"
		adapter.setNodes(updatedNode)
	}}
	commandService, err := command.NewService(store, authorizer, executor, registry, cache)
	if err != nil {
		t.Fatalf("create Command Service: %v", err)
	}
	commandService.SetNodeRefresh(synchronizer.Refresh)
	messageDispatcher := dispatcher.NewDispatcherWithCacheAndCapabilities(cache, resolver)
	if err := messageDispatcher.RegisterAuthenticated(protocol.MessageCommand, commandService); err != nil {
		t.Fatalf("register authenticated command handler: %v", err)
	}

	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithNodeStreamsAndDispatcher(socketPath, streams, messageDispatcher)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = server.Serve(ctx) }()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	encoder := protocol.NewEncoder(client)
	decoder := protocol.NewDecoder(client)
	snapshot, err := decoder.Decode()
	if err != nil || snapshot.Type != protocol.MessageEvent {
		t.Fatalf("initial stream message=%#v err=%v; want Node snapshot event", snapshot, err)
	}
	request := protocol.NewEnvelope("IPC-COMMAND-1", protocol.MessageCommand, json.RawMessage(`{"commandId":"cmd-ipc-1","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"DRAIN","reason":"scheduled maintenance"}}`))
	if err := encoder.Encode(request); err != nil {
		t.Fatalf("send command: %v", err)
	}
	var commandResult protocol.NodeCommandResponsePayload
	var initialResponse protocol.Envelope
	gotCommandResponse := false
	gotNodeChange := false
	for range 2 {
		response, err := decoder.Decode()
		if err != nil {
			t.Fatalf("read command response or Node event: %v", err)
		}
		switch response.Type {
		case protocol.MessageCommandResponse:
			if response.MessageID != request.MessageID || json.Unmarshal(response.Payload, &commandResult) != nil || commandResult.Status != protocol.CommandStatusSucceeded {
				t.Fatalf("command response=%#v result=%#v; want correlated success", response, commandResult)
			}
			initialResponse = response
			gotCommandResponse = true
		case protocol.MessageEvent:
			var event resource.NodeChangeBatch
			if err := json.Unmarshal(response.Payload, &event); err != nil {
				t.Fatalf("decode authoritative Node event: %v", err)
			}
			if event.Resource != "node" || event.Event != "changes" || event.Generation != 2 || len(event.Changes) != 1 || event.Changes[0].Node.Identity() != "c21" || event.Changes[0].Node.Status.State != "DRAIN" {
				t.Fatalf("Node change event=%#v; want committed c21 DRAIN at generation 2", event)
			}
			gotNodeChange = true
		default:
			t.Fatalf("unexpected IPC response type %q", response.Type)
		}
	}
	if !gotCommandResponse || !gotNodeChange {
		t.Fatalf("received command response=%t Node change=%t; want both", gotCommandResponse, gotNodeChange)
	}

	replay := request
	replay.Timestamp = "later"
	replay.Payload = json.RawMessage(`{ "commandId" : "cmd-ipc-1", "resource" : "node", "command" : "update", "target" : { "nodeName" : "c21" }, "parameters" : { "state" : "DRAIN", "reason" : "scheduled maintenance" } }`)
	if err := encoder.Encode(replay); err != nil {
		t.Fatalf("send same-message replay: %v", err)
	}
	replayed, err := decoder.Decode()
	if err != nil || replayed.Type != initialResponse.Type || string(replayed.Payload) != string(initialResponse.Payload) {
		t.Fatalf("replayed response=%#v err=%v; want original response", replayed, err)
	}

	conflict := request
	conflict.Payload = json.RawMessage(`{"commandId":"cmd-ipc-2","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"RESUME"}}`)
	if err := encoder.Encode(conflict); err != nil {
		t.Fatalf("send conflicting message ID: %v", err)
	}
	conflictResponse, err := decoder.Decode()
	if err != nil {
		t.Fatalf("read conflict response: %v", err)
	}
	var conflictError protocol.ErrorPayload
	if conflictResponse.Type != protocol.MessageError || json.Unmarshal(conflictResponse.Payload, &conflictError) != nil || conflictError.Code != protocol.ErrorCodeMessageIDConflict {
		t.Fatalf("conflict response=%#v error=%#v; want message-id-conflict", conflictResponse, conflictError)
	}

	executor.mu.Lock()
	callCount, operation, nodeName, reason := executor.calls, executor.operation, executor.node, executor.reason
	executor.mu.Unlock()
	resolver.mu.Lock()
	resolveCount := resolver.calls
	resolver.mu.Unlock()
	if callCount != 1 || operation != authorization.OperationNodeDrain || nodeName != "c21" || reason != "scheduled maintenance" || resolveCount != 1 {
		t.Fatalf("executor calls=%d operation=%s node=%s reason=%q resolver calls=%d; want one authorized drain", callCount, operation, nodeName, reason, resolveCount)
	}

	peer, err := connection.NewConnection(client)
	if err != nil {
		t.Fatalf("resolve test client's peer identity: %v", err)
	}
	defer peer.Close()
	record, found, err := store.Lookup(context.Background(), peer.User.UID, "cmd-ipc-1")
	if err != nil || !found || record.Status != ledger.StatusSucceeded || record.RequestingUsername != peer.User.Username || record.RequestingGID != peer.User.GID || record.RequestMessageID != request.MessageID || record.ServiceUID != uint32(os.Geteuid()) || record.ServiceUsername == "" {
		t.Fatalf("command ledger record=%#v found=%v err=%v; want succeeded caller record", record, found, err)
	}
}

func TestUnauthorizedCommandThroughIPCIsPersistedAndNotExecuted(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("c21", 1)})
	registry, err := authorization.NewNodeRegistry()
	if err != nil {
		t.Fatalf("create Node registry: %v", err)
	}
	store, err := ledger.Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("open test ledger: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	resolver := &nonAdminCapabilityResolver{}
	authorizer := authorization.NewAuthorizer(registry, resolver)
	executor := &ipcNodeExecutor{}
	commandService, err := command.NewService(store, authorizer, executor, registry, cache)
	if err != nil {
		t.Fatalf("create Command Service: %v", err)
	}
	messageDispatcher := dispatcher.NewDispatcherWithCacheAndCapabilities(cache, resolver)
	if err := messageDispatcher.RegisterAuthenticated(protocol.MessageCommand, commandService); err != nil {
		t.Fatalf("register authenticated command handler: %v", err)
	}

	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithDispatcher(socketPath, messageDispatcher)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = server.Serve(ctx) }()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	request := protocol.NewEnvelope("IPC-DENY-1", protocol.MessageCommand, json.RawMessage(`{"commandId":"cmd-ipc-denied","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"DRAIN","reason":"scheduled maintenance"}}`))
	if err := protocol.NewEncoder(client).Encode(request); err != nil {
		t.Fatalf("send command: %v", err)
	}
	response, err := protocol.NewDecoder(client).Decode()
	if err != nil {
		t.Fatalf("read authorization response: %v", err)
	}
	var failure protocol.ErrorPayload
	if response.Type != protocol.MessageError || response.MessageID != request.MessageID || json.Unmarshal(response.Payload, &failure) != nil || failure.Code != protocol.ErrorCodeAuthorizationDenied || failure.Category != protocol.ErrorCategoryAuthorization {
		t.Fatalf("response=%#v failure=%#v; want correlated authorization-denied", response, failure)
	}

	peer, err := connection.NewConnection(client)
	if err != nil {
		t.Fatalf("resolve test client's peer identity: %v", err)
	}
	defer peer.Close()
	record, found, err := store.Lookup(context.Background(), peer.User.UID, "cmd-ipc-denied")
	if err != nil || !found || record.Status != ledger.StatusDenied || executor.calls != 0 {
		t.Fatalf("record=%#v found=%v err=%v executor calls=%d; want denied tombstone and no execution", record, found, err, executor.calls)
	}
	resolver.mu.Lock()
	resolveCount := resolver.calls
	resolver.mu.Unlock()
	if resolveCount != 1 {
		t.Fatalf("authorization resolver calls = %d, want 1", resolveCount)
	}
}

func TestLedgerUncertaintyThroughIPCBlocksExecution(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("c21", 1)})
	registry, err := authorization.NewNodeRegistry()
	if err != nil {
		t.Fatalf("create Node registry: %v", err)
	}
	store, err := ledger.Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("open test ledger: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	faultLedger := &ipcTransitionFaultLedger{Ledger: store}
	resolver := &adminCapabilityResolver{}
	authorizer := authorization.NewAuthorizer(registry, resolver)
	executor := &ipcNodeExecutor{}
	commandService, err := command.NewService(faultLedger, authorizer, executor, registry, cache)
	if err != nil {
		t.Fatalf("create Command Service: %v", err)
	}
	messageDispatcher := dispatcher.NewDispatcherWithCacheAndCapabilities(cache, resolver)
	if err := messageDispatcher.RegisterAuthenticated(protocol.MessageCommand, commandService); err != nil {
		t.Fatalf("register authenticated command handler: %v", err)
	}

	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithDispatcher(socketPath, messageDispatcher)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = server.Serve(ctx) }()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	request := protocol.NewEnvelope("IPC-LEDGER-ERROR-1", protocol.MessageCommand, json.RawMessage(`{"commandId":"cmd-ipc-ledger-error","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"DRAIN","reason":"scheduled maintenance"}}`))
	if err := protocol.NewEncoder(client).Encode(request); err != nil {
		t.Fatalf("send command: %v", err)
	}
	response, err := protocol.NewDecoder(client).Decode()
	if err != nil {
		t.Fatalf("read persistence error response: %v", err)
	}
	var failure protocol.ErrorPayload
	if response.Type != protocol.MessageError || response.MessageID != request.MessageID || json.Unmarshal(response.Payload, &failure) != nil || failure.Code != protocol.ErrorCodeLedgerPersistenceFailed || failure.Retryable {
		t.Fatalf("response=%#v failure=%#v; want correlated non-retryable ledger error", response, failure)
	}
	if executor.calls != 0 || !faultLedger.AdmissionBlocked() {
		t.Fatalf("executor calls=%d admission blocked=%v; want no execution and blocked admission", executor.calls, faultLedger.AdmissionBlocked())
	}
	peer, err := connection.NewConnection(client)
	if err != nil {
		t.Fatalf("resolve test client's peer identity: %v", err)
	}
	defer peer.Close()
	record, found, err := store.Lookup(context.Background(), peer.User.UID, "cmd-ipc-ledger-error")
	if err != nil || !found || record.Status != ledger.StatusAuthorizationPending {
		t.Fatalf("record=%#v found=%v err=%v; want unresolved non-executable claim", record, found, err)
	}
}

func TestFreshAuthorizationChangesOnOpenIPCConnection(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("c21", 1)})
	registry, err := authorization.NewNodeRegistry()
	if err != nil {
		t.Fatalf("create Node registry: %v", err)
	}
	store, err := ledger.Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("open test ledger: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	resolver := &changeableCapabilityResolver{admin: true}
	authorizer := authorization.NewAuthorizer(registry, resolver)
	executor := &ipcNodeExecutor{}
	commandService, err := command.NewService(store, authorizer, executor, registry, cache)
	if err != nil {
		t.Fatalf("create Command Service: %v", err)
	}
	messageDispatcher := dispatcher.NewDispatcherWithCacheAndCapabilities(cache, resolver)
	if err := messageDispatcher.RegisterAuthenticated(protocol.MessageCommand, commandService); err != nil {
		t.Fatalf("register authenticated command handler: %v", err)
	}

	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithDispatcher(socketPath, messageDispatcher)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = server.Serve(ctx) }()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	peer, err := connection.NewConnection(client)
	if err != nil {
		t.Fatalf("resolve test client's peer identity: %v", err)
	}
	defer peer.Close()
	if err := client.SetDeadline(time.Now().Add(4 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	encoder := protocol.NewEncoder(client)
	decoder := protocol.NewDecoder(client)
	send := func(messageID, commandID, state string) protocol.Envelope {
		parameters := protocol.NodeCommandParameters{State: state}
		if state == "DRAIN" {
			reason := "maintenance"
			parameters.Reason = &reason
		}
		payload, err := json.Marshal(protocol.NodeCommandRequestPayload{
			CommandID: commandID, Resource: "node", Command: "update",
			Target: protocol.NodeCommandTarget{NodeName: "c21"}, Parameters: parameters,
		})
		if err != nil {
			t.Fatalf("marshal %s command: %v", commandID, err)
		}
		message := protocol.NewEnvelope(messageID, protocol.MessageCommand, payload)
		if err := encoder.Encode(message); err != nil {
			t.Fatalf("send %s command: %v", commandID, err)
		}
		response, err := decoder.Decode()
		if err != nil {
			t.Fatalf("read %s response: %v", commandID, err)
		}
		return response
	}

	allowed := send("ROLE-CHANGE-1", "cmd-role-admin", "DRAIN")
	if allowed.Type != protocol.MessageCommandResponse {
		t.Fatalf("initial admin response=%#v; want allowed command", allowed)
	}
	resolver.mu.Lock()
	resolver.admin = false
	resolver.mu.Unlock()

	denied := send("ROLE-CHANGE-2", "cmd-role-revoked", "RESUME")
	var denial protocol.ErrorPayload
	if denied.Type != protocol.MessageError || json.Unmarshal(denied.Payload, &denial) != nil || denial.Code != protocol.ErrorCodeAuthorizationDenied {
		t.Fatalf("revoked-role response=%#v denial=%#v; want authorization-denied", denied, denial)
	}
	deniedRecord, found, err := store.Lookup(context.Background(), peer.User.UID, "cmd-role-revoked")
	if err != nil || !found || deniedRecord.Status != ledger.StatusDenied {
		t.Fatalf("denied record=%#v found=%v err=%v; want durable denied tombstone", deniedRecord, found, err)
	}

	resolver.mu.Lock()
	resolver.admin = true
	resolver.mu.Unlock()
	deniedRetry := send("ROLE-CHANGE-3", "cmd-role-revoked", "RESUME")
	if deniedRetry.Type != protocol.MessageError || json.Unmarshal(deniedRetry.Payload, &denial) != nil || denial.Code != protocol.ErrorCodeAuthorizationDenied {
		t.Fatalf("same-ID retry after grant=%#v denial=%#v; want stored denial", deniedRetry, denial)
	}
	newIDAllowed := send("ROLE-CHANGE-4", "cmd-role-admin-new", "RESUME")
	if newIDAllowed.Type != protocol.MessageCommandResponse {
		t.Fatalf("new-ID response after grant=%#v; want allowed command", newIDAllowed)
	}

	executor.mu.Lock()
	executorCalls := executor.calls
	executor.mu.Unlock()
	resolver.mu.Lock()
	resolverCalls := resolver.calls
	resolver.mu.Unlock()
	if executorCalls != 2 || resolverCalls != 3 {
		t.Fatalf("executor calls=%d resolver calls=%d; want 2 executions and 3 fresh authorizations", executorCalls, resolverCalls)
	}
}

func TestCapabilitiesRequestUsesPeerIdentityOverIPC(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 0)})
	registry, err := stream.NewNodeStreamRegistry(cache, stream.DefaultNodeStreamQueueSize)
	if err != nil {
		t.Fatalf("NewNodeStreamRegistry returned error: %v", err)
	}
	resolver := &capabilityResolverForIPC{requester: make(chan connection.UserContext, 1)}
	messageDispatcher := dispatcher.NewDispatcherWithCacheAndCapabilities(cache, resolver)
	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithNodeStreamsAndDispatcher(socketPath, registry, messageDispatcher)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = server.Serve(ctx) }()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	decoder := protocol.NewDecoder(client)
	if _, err := decoder.Decode(); err != nil {
		t.Fatalf("decode initial Node snapshot: %v", err)
	}
	request := protocol.NewEnvelope("CAP-IPC-1", protocol.MessageCapabilitiesRequest, json.RawMessage(`{}`))
	if err := protocol.NewEncoder(client).Encode(request); err != nil {
		t.Fatalf("encode capabilities request: %v", err)
	}
	response, err := decoder.Decode()
	if err != nil {
		t.Fatalf("decode capabilities response: %v", err)
	}
	if response.Type != protocol.MessageCapabilitiesResponse || response.MessageID != request.MessageID {
		t.Fatalf("response = %#v, want correlated capabilities response", response)
	}
	resolved, err := protocol.DecodeCapabilitiesResponsePayload(response.Payload)
	if err != nil || !resolved.Capabilities.NodesDrain || !resolved.Capabilities.NodesResume {
		t.Fatalf("capability payload = %#v, err = %v", resolved, err)
	}
	select {
	case requester := <-resolver.requester:
		if requester.UID != uint32(os.Getuid()) || requester.GID != uint32(os.Getgid()) || requester.Username == "" {
			t.Fatalf("resolver requester = %#v, want peer uid=%d gid=%d with username", requester, os.Getuid(), os.Getgid())
		}
	case <-time.After(time.Second):
		t.Fatal("resolver was not called")
	}
}

func TestConcurrentCapabilitiesRequestsPreserveCorrelation(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 0)})
	registry, err := stream.NewNodeStreamRegistry(cache, stream.DefaultNodeStreamQueueSize)
	if err != nil {
		t.Fatalf("NewNodeStreamRegistry returned error: %v", err)
	}
	resolver := &capabilityResolverForIPC{requester: make(chan connection.UserContext, 2)}
	messageDispatcher := dispatcher.NewDispatcherWithCacheAndCapabilities(cache, resolver)
	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithNodeStreamsAndDispatcher(socketPath, registry, messageDispatcher)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = server.Serve(ctx) }()

	client, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	decoder := protocol.NewDecoder(client)
	if _, err := decoder.Decode(); err != nil {
		t.Fatalf("decode initial Node snapshot: %v", err)
	}
	requests := []protocol.Envelope{
		protocol.NewEnvelope("CAP-CONCURRENT-1", protocol.MessageCapabilitiesRequest, json.RawMessage(`{}`)),
		protocol.NewEnvelope("CAP-CONCURRENT-2", protocol.MessageCapabilitiesRequest, json.RawMessage(`{}`)),
	}
	encoder := protocol.NewEncoder(client)
	for _, request := range requests {
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode capabilities request %q: %v", request.MessageID, err)
		}
	}

	got := make(map[string]bool, len(requests))
	for range len(requests) {
		response, err := decoder.Decode()
		if err != nil {
			t.Fatalf("decode capabilities response: %v", err)
		}
		if response.Type != protocol.MessageCapabilitiesResponse {
			t.Fatalf("response type = %q, want capabilities-response", response.Type)
		}
		if response.MessageID != requests[0].MessageID && response.MessageID != requests[1].MessageID {
			t.Fatalf("unexpected response messageId %q", response.MessageID)
		}
		if got[response.MessageID] {
			t.Fatalf("duplicate response messageId %q", response.MessageID)
		}
		capabilities, err := protocol.DecodeCapabilitiesResponsePayload(response.Payload)
		if err != nil || !capabilities.Capabilities.NodesDrain || !capabilities.Capabilities.NodesResume {
			t.Fatalf("response %q capabilities=%#v err=%v", response.MessageID, capabilities, err)
		}
		got[response.MessageID] = true
	}
	for _, request := range requests {
		if !got[request.MessageID] {
			t.Errorf("no correlated response for %q", request.MessageID)
		}
	}
	for range len(requests) {
		select {
		case requester := <-resolver.requester:
			if requester.UID != uint32(os.Getuid()) || requester.GID != uint32(os.Getgid()) || requester.Username == "" {
				t.Fatalf("resolver requester = %#v, want server-derived peer identity", requester)
			}
		case <-time.After(time.Second):
			t.Fatal("resolver did not receive both capability requests")
		}
	}
}

func TestNodeStreamEndToEndTracksSynchronizationAndClientLifecycle(t *testing.T) {
	initialNodes := []resource.Node{
		newTestNode("node001", "IDLE"),
		newTestNode("node002", "IDLE"),
	}
	adapter := &mutableNodeAdapter{nodes: initialNodes}
	cache := resource.NewNodeCache()
	registry, err := stream.NewNodeStreamRegistry(cache, stream.DefaultNodeStreamQueueSize)
	if err != nil {
		t.Fatalf("NewNodeStreamRegistry returned error: %v", err)
	}
	synchronizer, err := resource.NewNodeSynchronizer(adapter, cache, registry, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("NewNodeSynchronizer returned error: %v", err)
	}
	if err := synchronizer.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh returned error: %v", err)
	}

	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServerWithNodeStreams(socketPath, registry)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ctx)
	}()

	syncCtx, stopSync := context.WithCancel(context.Background())
	t.Cleanup(stopSync)
	syncErr := make(chan error, 1)
	go func() {
		syncErr <- synchronizer.Run(syncCtx)
	}()

	firstClient := dialNodeStream(t, socketPath)
	secondClient := dialNodeStream(t, socketPath)
	for _, client := range []*nodeStreamTestClient{firstClient, secondClient} {
		snapshot := readNodeStreamPayload(t, client)
		if snapshot.Event != "snapshot" || snapshot.Generation != 1 || len(snapshot.Nodes) != 2 {
			t.Fatalf("initial Node stream payload = %#v, want generation 1 snapshot with 2 Nodes", snapshot)
		}
	}

	adapter.setNodes(newTestNode("node001", "DOWN"), newTestNode("node003", "IDLE"))
	for _, client := range []*nodeStreamTestClient{firstClient, secondClient} {
		batch := readNodeStreamPayload(t, client)
		if batch.Event != "changes" || batch.Generation != 2 {
			t.Fatalf("Node change payload = %#v, want generation 2 changes", batch)
		}
		if got := changeKindsByIdentity(batch.Changes); got["node001"] != "updated" ||
			got["node002"] != "removed" || got["node003"] != "added" {
			t.Fatalf("change kinds = %#v, want update node001, remove node002, add node003", got)
		}
	}

	if err := firstClient.conn.Close(); err != nil {
		t.Fatalf("close first client: %v", err)
	}
	waitForActiveStreams(t, registry, 1)

	adapter.setNodes(newTestNode("node003", "DOWN"))
	secondBatch := readNodeStreamPayload(t, secondClient)
	if secondBatch.Event != "changes" || secondBatch.Generation != 3 {
		t.Fatalf("surviving client payload = %#v, want generation 3 changes", secondBatch)
	}
	if got := changeKindsByIdentity(secondBatch.Changes); got["node001"] != "removed" ||
		got["node003"] != "updated" {
		t.Fatalf("surviving client changes = %#v, want remove node001 and update node003", got)
	}
	if got := cache.Generation(); got != 3 {
		t.Fatalf("cache generation = %d, want 3 after disconnected client", got)
	}

	reconnectedClient := dialNodeStream(t, socketPath)
	freshSnapshot := readNodeStreamPayload(t, reconnectedClient)
	if freshSnapshot.Event != "snapshot" || freshSnapshot.Generation != 3 ||
		len(freshSnapshot.Nodes) != 1 || freshSnapshot.Nodes[0].Identity() != "node003" ||
		freshSnapshot.Nodes[0].Status.State != "DOWN" {
		t.Fatalf("reconnected client snapshot = %#v, want authoritative node003 DOWN at generation 3", freshSnapshot)
	}

	_ = secondClient.conn.Close()
	_ = reconnectedClient.conn.Close()
	waitForActiveStreams(t, registry, 0)

	stopSync()
	select {
	case err := <-syncErr:
		if err != nil {
			t.Fatalf("NodeSynchronizer.Run returned error after cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("NodeSynchronizer.Run did not stop after cancellation")
	}

	if err := server.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve returned error after close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not exit after close")
	}
}

type nodeStreamTestClient struct {
	conn    net.Conn
	decoder *protocol.Decoder
}

type nodeStreamTestPayload struct {
	Resource   string                `json:"resource"`
	Event      string                `json:"event"`
	Generation uint64                `json:"generation"`
	Nodes      []resource.Node       `json:"nodes"`
	Changes    []resource.NodeChange `json:"changes"`
}

func dialNodeStream(t *testing.T, socketPath string) *nodeStreamTestClient {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial Node stream: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return &nodeStreamTestClient{conn: conn, decoder: protocol.NewDecoder(conn)}
}

func readNodeStreamPayload(t *testing.T, client *nodeStreamTestClient) nodeStreamTestPayload {
	t.Helper()
	if err := client.conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set Node stream read deadline: %v", err)
	}
	envelope, err := client.decoder.Decode()
	if err != nil {
		t.Fatalf("decode Node stream envelope: %v", err)
	}
	if envelope.Type != protocol.MessageEvent {
		t.Fatalf("Node stream message type = %q, want event", envelope.Type)
	}
	var payload nodeStreamTestPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("decode Node stream payload: %v", err)
	}
	if payload.Resource != "node" {
		t.Fatalf("Node stream resource = %q, want node", payload.Resource)
	}
	return payload
}

func newTestNode(name, state string) resource.Node {
	node := resource.NewNode(name, 0)
	node.Status.State = state
	return node
}

func changeKindsByIdentity(changes []resource.NodeChange) map[string]resource.NodeChangeKind {
	kinds := make(map[string]resource.NodeChangeKind, len(changes))
	for _, change := range changes {
		kinds[change.Node.Identity()] = change.Kind
	}
	return kinds
}

func waitForActiveStreams(t *testing.T, registry *stream.NodeStreamRegistry, want int) {
	t.Helper()
	deadline := time.After(time.Second)
	for registry.ActiveCount() != want {
		select {
		case <-deadline:
			t.Fatalf("active Node streams = %d, want %d", registry.ActiveCount(), want)
		case <-time.After(time.Millisecond):
		}
	}
}

func waitForStreamCleanup(t *testing.T, registry *stream.NodeStreamRegistry) {
	t.Helper()
	deadline := time.After(time.Second)
	for registry.ActiveCount() != 0 {
		select {
		case <-deadline:
			t.Fatal("disconnected stream remained registered")
		case <-time.After(time.Millisecond):
		}
	}
}
