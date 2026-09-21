package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
)

func TestServerDoesNotStoreCancelFunc(t *testing.T) {
	if _, ok := reflect.TypeOf(Server{}).FieldByName("cancel"); ok {
		t.Fatal("Server should not store a context.CancelFunc in Phase 1B")
	}
}

func TestServerRejectsActiveSocket(t *testing.T) {
	socketDir := filepath.Join(t.TempDir(), "ipc")
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() returned error: %v", err)
	}

	socketPath := filepath.Join(socketDir, "cockpit-slurm.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen() returned error: %v", err)
	}
	defer listener.Close()

	server := NewServer(socketPath)
	if err := server.Listen(); err == nil {
		t.Fatal("Listen() should fail when another process is already listening")
	}
}

func TestServerUsesEnvironmentSocketOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom", "cockpit-slurm.sock")
	t.Setenv("COCKPIT_SLURM_SOCKET_PATH", want)

	server := NewServer("")
	if got := server.SocketPath(); got != want {
		t.Fatalf("SocketPath() = %q, want %q", got, want)
	}
}

func TestServerListenAndClose(t *testing.T) {
	socketDir := filepath.Join(t.TempDir(), "ipc")
	socketPath := filepath.Join(socketDir, "cockpit-slurm.sock")

	server := NewServer(socketPath)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen() returned error: %v", err)
	}
	defer func() {
		_ = server.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ctx)
	}()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Dial() returned error: %v", err)
	}

	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	if err := server.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve() returned error after close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve() did not exit after Close()")
	}

	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("expected socket to be removed after close, stat error = %v", err)
	}
}

func TestServerDispatchesHelloOverSocket(t *testing.T) {
	socketDir := filepath.Join(t.TempDir(), "ipc")
	socketPath := filepath.Join(socketDir, "cockpit-slurm.sock")

	server := NewServer(socketPath)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen() returned error: %v", err)
	}
	defer func() {
		_ = server.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ctx)
	}()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Dial() returned error: %v", err)
	}
	defer conn.Close()

	msg := protocol.NewEnvelope(
		"MSG001",
		protocol.MessageHello,
		json.RawMessage(`{"client":"cockpit-slurm-channel","clientVersion":"0.1.0"}`),
	)

	enc := protocol.NewEncoder(conn)
	if err := enc.Encode(msg); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	dec := protocol.NewDecoder(conn)
	resp, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode(response) returned error: %v", err)
	}

	if resp.Type != protocol.MessageHelloResponse {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessageHelloResponse)
	}
	if err := protocol.ValidateEnvelope(resp); err != nil {
		t.Fatalf("ValidateEnvelope(response) returned error: %v", err)
	}

	if server.connections.Count() != 1 {
		t.Fatalf("active connection count = %d, want 1", server.connections.Count())
	}

	if err := server.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve() returned error after close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve() did not exit after Close()")
	}
}

func TestServerDispatchesNodesQueryOverSocket(t *testing.T) {
	socketDir := filepath.Join(t.TempDir(), "ipc")
	socketPath := filepath.Join(socketDir, "cockpit-slurm.sock")

	server := NewServer(socketPath)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen() returned error: %v", err)
	}
	defer func() {
		_ = server.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ctx)
	}()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Dial() returned error: %v", err)
	}
	defer conn.Close()

	msg := protocol.NewEnvelope(
		"MSG-NODES-1",
		protocol.MessageQuery,
		json.RawMessage(`{"resource":"nodes"}`),
	)
	if err := protocol.NewEncoder(conn).Encode(msg); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	resp, err := protocol.NewDecoder(conn).Decode()
	if err != nil {
		t.Fatalf("Decode(response) returned error: %v", err)
	}
	if resp.Type != protocol.MessageQueryResponse {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessageQueryResponse)
	}
	if resp.MessageID != msg.MessageID {
		t.Fatalf("MessageID = %q, want %q", resp.MessageID, msg.MessageID)
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal(response.Payload) returned error: %v", err)
	}
	if payload["resource"] != "nodes" {
		t.Fatalf("payload resource = %v, want %q", payload["resource"], "nodes")
	}
	if payload["generation"] == nil {
		t.Fatal("generation is missing from query response")
	}
	if payload["nodes"] == nil {
		t.Fatal("nodes are missing from query response")
	}

	if err := server.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve() returned error after close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve() did not exit after Close()")
	}
}

func TestServerHandlesConcurrentPingRequests(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "cockpit-slurm.sock")
	server := NewServer(socketPath)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen() returned error: %v", err)
	}
	defer func() {
		_ = server.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(ctx)
	}()

	const clientCount = 4
	results := make(chan error, clientCount)
	for client := 0; client < clientCount; client++ {
		go func(client int) {
			conn, err := net.Dial("unix", socketPath)
			if err != nil {
				results <- fmt.Errorf("client %d dial: %w", client, err)
				return
			}
			defer conn.Close()

			messageID := fmt.Sprintf("PING-%d", client)
			if err := protocol.NewEncoder(conn).Encode(protocol.NewEnvelope(messageID, protocol.MessagePing, json.RawMessage(`{}`))); err != nil {
				results <- fmt.Errorf("client %d encode: %w", client, err)
				return
			}

			response, err := protocol.NewDecoder(conn).Decode()
			if err != nil {
				results <- fmt.Errorf("client %d decode: %w", client, err)
				return
			}
			if response.Type != protocol.MessagePong {
				results <- fmt.Errorf("client %d response type = %q, want %q", client, response.Type, protocol.MessagePong)
				return
			}
			if response.MessageID != messageID {
				results <- fmt.Errorf("client %d message ID = %q, want %q", client, response.MessageID, messageID)
				return
			}
			results <- nil
		}(client)
	}

	for client := 0; client < clientCount; client++ {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent ping responses")
		}
	}

	if err := server.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("Serve() returned error after close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve() did not exit after Close()")
	}
}
