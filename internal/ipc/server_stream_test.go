package ipc

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/dispatcher"
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
