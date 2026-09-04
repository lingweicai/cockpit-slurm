package ipc

import (
	"context"
	"encoding/json"
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

	socketPath := filepath.Join(socketDir, "bridge.sock")
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

func TestServerListenAndClose(t *testing.T) {
	socketDir := filepath.Join(t.TempDir(), "ipc")
	socketPath := filepath.Join(socketDir, "bridge.sock")

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
	socketPath := filepath.Join(socketDir, "bridge.sock")

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
