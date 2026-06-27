package channelrelay

import (
	"bytes"
	"io"
	"net"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultSocketPathUsesEnvironment(t *testing.T) {
	t.Setenv("COCKPIT_SLURM_BRIDGE_SOCKET_PATH", "/tmp/custom.sock")
	if got := DefaultSocketPath(); got != "/tmp/custom.sock" {
		t.Fatalf("DefaultSocketPath() = %q, want %q", got, "/tmp/custom.sock")
	}
}

func TestRunRelaysBytesToUnixSocket(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "bridge.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 64)
		n, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			return
		}
		if _, err := conn.Write([]byte("pong")); err != nil {
			return
		}
		_ = conn.(*net.UnixConn).CloseWrite()
		_ = n
	}()

	var stdout bytes.Buffer
	if err := Run(bytes.NewBufferString("ping"), &stdout, socketPath); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := stdout.String(); got != "pong" {
		t.Fatalf("stdout = %q, want %q", got, "pong")
	}

	<-serverDone
}

func TestDefaultSocketPathFallsBackToRuntimeDirOrUID(t *testing.T) {
	t.Setenv("COCKPIT_SLURM_BRIDGE_SOCKET_PATH", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	got := DefaultSocketPath()
	if got == "" {
		t.Fatal("DefaultSocketPath() returned empty path")
	}
	if runtime.GOOS != "windows" && filepath.Base(got) != "bridge.sock" {
		t.Fatalf("DefaultSocketPath() = %q, want a bridge.sock path", got)
	}
}
