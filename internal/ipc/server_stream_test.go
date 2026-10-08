package ipc

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
	"github.com/lingweicai/cockpit-slurm/internal/stream"
)

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
