package ipc

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
	"github.com/lingweicai/cockpit-slurm/internal/stream"
)

type mutableNodeAdapter struct {
	mu    sync.RWMutex
	nodes []resource.Node
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
