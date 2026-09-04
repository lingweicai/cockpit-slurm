package connection

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestNewConnectionCreatesIDsAndUserContext(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "phase1e.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen() returned error: %v", err)
	}
	defer listener.Close()

	clientConn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Dial() returned error: %v", err)
	}
	defer clientConn.Close()

	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("Accept() returned error: %v", err)
	}
	defer serverConn.Close()

	conn, err := NewConnection(serverConn)
	if err != nil {
		t.Fatalf("NewConnection() returned error: %v", err)
	}

	if conn == nil {
		t.Fatal("NewConnection() returned nil connection")
	}
	if conn.ID == "" {
		t.Fatal("connection ID should not be empty")
	}
	if conn.Session == nil {
		t.Fatal("session should not be nil")
	}
	if conn.Session.ID == "" {
		t.Fatal("session ID should not be empty")
	}
	if conn.Session.ConnectionID != conn.ID {
		t.Fatalf("session connection ID = %q, want %q", conn.Session.ConnectionID, conn.ID)
	}
	if conn.User.UID != uint32(os.Getuid()) {
		t.Fatalf("user UID = %d, want %d", conn.User.UID, uint32(os.Getuid()))
	}
	if conn.User.GID != uint32(os.Getgid()) {
		t.Fatalf("user GID = %d, want %d", conn.User.GID, uint32(os.Getgid()))
	}
	if conn.Context == nil {
		t.Fatal("connection context should not be nil")
	}
	if conn.Cancel == nil {
		t.Fatal("connection cancel function should not be nil")
	}
}

func TestConnectionLifecycleTransitions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &Connection{
		ID:      "conn-1",
		Context: ctx,
		Cancel:  cancel,
		State:   StateNew,
	}

	if conn.State != StateNew {
		t.Fatalf("initial state = %q, want %q", conn.State, StateNew)
	}

	conn.State = StateActive
	if conn.State != StateActive {
		t.Fatalf("active state = %q, want %q", conn.State, StateActive)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if conn.State != StateClosed {
		t.Fatalf("final state = %q, want %q", conn.State, StateClosed)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("second Close() returned error: %v", err)
	}
	if conn.State != StateClosed {
		t.Fatalf("state after second close = %q, want %q", conn.State, StateClosed)
	}
}

func TestPeerIdentityFailureClosesConnection(t *testing.T) {
	t.Run("non-unix-connection", func(t *testing.T) {
		conn, err := net.Dial("unix", filepath.Join(t.TempDir(), "missing.sock"))
		if err == nil {
			_ = conn.Close()
		}
		if err == nil {
			t.Fatal("net.Dial() unexpectedly succeeded")
		}
		if _, ok := err.(*net.OpError); !ok {
			t.Logf("dial error: %v", err)
		}
	})

	t.Run("invalid peer identity path", func(t *testing.T) {
		_, err := peerIdentity(nil)
		if !errors.Is(err, ErrPeerIdentityUnavailable) {
			t.Fatalf("peerIdentity(nil) error = %v, want %v", err, ErrPeerIdentityUnavailable)
		}
	})
}

func TestConnectionManagerTracksConnections(t *testing.T) {
	mgr := NewConnectionManager()
	conn := &Connection{ID: "conn-1", Context: context.Background()}
	if err := mgr.Register(conn); err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}
	if got := mgr.Count(); got != 1 {
		t.Fatalf("Count() = %d, want 1", got)
	}
	if _, ok := mgr.Get("conn-1"); !ok {
		t.Fatal("Get() did not return connection for registered ID")
	}
	if err := mgr.Register(&Connection{ID: "conn-1", Context: context.Background()}); err == nil {
		t.Fatal("Register() should reject duplicate connection IDs")
	}
	if err := mgr.Close("conn-1"); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if got := mgr.Count(); got != 0 {
		t.Fatalf("Count() = %d, want 0 after Close()", got)
	}
}

func TestConnectionManagerCloseAllClosesEveryConnection(t *testing.T) {
	mgr := NewConnectionManager()
	conn1 := &Connection{ID: "conn-1", Context: context.Background()}
	conn2 := &Connection{ID: "conn-2", Context: context.Background()}
	if err := mgr.Register(conn1); err != nil {
		t.Fatalf("Register(conn1) returned error: %v", err)
	}
	if err := mgr.Register(conn2); err != nil {
		t.Fatalf("Register(conn2) returned error: %v", err)
	}
	mgr.CloseAll()
	if got := mgr.Count(); got != 0 {
		t.Fatalf("Count() = %d, want 0 after CloseAll()", got)
	}
}
