package ipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestServerDoesNotStoreCancelFunc(t *testing.T) {
	if _, ok := reflect.TypeOf(Server{}).FieldByName("cancel"); ok {
		t.Fatal("Server should not store a context.CancelFunc in Phase 1B")
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
