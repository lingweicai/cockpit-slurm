package channel

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
)

func TestChannelRoundTripPreservesMessageIDAndPayload(t *testing.T) {
	var buf bytes.Buffer
	msg := protocol.NewEnvelope("MSG101", protocol.MessageHello, []byte(`{"client":"cockpit-slurm-channel"}`))

	enc := protocol.NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	dec := protocol.NewDecoder(&buf)
	resp, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode() returned error: %v", err)
	}
	if resp.MessageID != "MSG101" {
		t.Fatalf("MessageID = %q, want %q", resp.MessageID, "MSG101")
	}
	if string(resp.Payload) != `{"client":"cockpit-slurm-channel"}` {
		t.Fatalf("Payload = %s, want %s", resp.Payload, `{"client":"cockpit-slurm-channel"}`)
	}
}

func TestChannelSocketFailureClosesCleanly(t *testing.T) {
	badSocket := filepath.Join(t.TempDir(), "missing", "bridge.sock")
	if err := os.MkdirAll(filepath.Dir(badSocket), 0o755); err != nil {
		t.Fatalf("MkdirAll() returned error: %v", err)
	}

	_, err := Connect(badSocket)
	if err == nil {
		t.Fatal("Connect() should fail for an unavailable socket")
	}
}

func TestChannelCopyForwardsBytesWithoutMutation(t *testing.T) {
	ch := New()
	payload := []byte(`{"status":"ok"}`)
	src := bytes.NewReader(payload)
	dst := &bytes.Buffer{}

	if err := ch.copy(dst, src); err != nil {
		t.Fatalf("copy() returned error: %v", err)
	}
	if got := dst.Bytes(); !bytes.Equal(got, payload) {
		t.Fatalf("copy() = %s, want %s", got, payload)
	}
}

func TestChannelCopyReturnsNilWhenSourceCloses(t *testing.T) {
	ch := New()
	payload := []byte(`{"status":"ok"}`)
	reader, writer := io.Pipe()
	var dst bytes.Buffer

	go func() {
		defer writer.Close()
		_, _ = writer.Write(payload)
	}()

	if err := ch.copy(&dst, reader); err != nil {
		t.Fatalf("copy() returned error: %v", err)
	}
	if got := dst.Bytes(); !bytes.Equal(got, payload) {
		t.Fatalf("copy() = %s, want %s", got, payload)
	}
}

func TestChannelDisconnectsBothSidesOnBridgeShutdown(t *testing.T) {
	cockpitIn, cockpitOut := net.Pipe()
	bridgeIn, bridgeOut := net.Pipe()
	defer cockpitIn.Close()
	defer cockpitOut.Close()
	defer bridgeIn.Close()
	defer bridgeOut.Close()

	ch := New()
	errCh := make(chan error, 1)
	go func() {
		errCh <- ch.Run(cockpitIn, bridgeIn)
	}()

	if err := bridgeOut.Close(); err != nil {
		t.Fatalf("Close(bridgeOut) returned error: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && err != ErrClosed && err != io.EOF {
			t.Fatalf("Run() returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after bridge shutdown")
	}
}

func TestChannelConcurrentInstancesStayIsolated(t *testing.T) {
	first := New()
	second := New()

	if first == second {
		t.Fatal("New() returned the same channel instance")
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first.Close() returned error: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second.Close() returned error: %v", err)
	}
}

func TestChannelConnectsToUnixSocketAndReadsBridgeData(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "bridge.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen() returned error: %v", err)
	}
	defer listener.Close()

	payload := []byte("hello-from-bridge")
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		defer conn.Close()

		_, err = conn.Write(payload)
		acceptErr <- err
	}()

	conn, err := Connect(socketPath)
	if err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("ReadFull() returned error: %v", err)
	}
	if !bytes.Equal(buf, payload) {
		t.Fatalf("ReadFull() = %q, want %q", buf, payload)
	}

	if err := <-acceptErr; err != nil {
		t.Fatalf("listener.Accept() write error: %v", err)
	}
}

func TestChannelRelaysTrafficAcrossRealUnixSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "bridge.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen() returned error: %v", err)
	}
	defer listener.Close()

	bridgeAccepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		bridgeAccepted <- conn
	}()

	bridgeConn, err := Connect(socketPath)
	if err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}
	defer bridgeConn.Close()

	var bridgePeer net.Conn
	select {
	case bridgePeer = <-bridgeAccepted:
	case err := <-acceptErr:
		t.Fatalf("listener.Accept() returned error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("listener.Accept() timed out")
	}
	defer bridgePeer.Close()

	cockpitIn, cockpitOut := net.Pipe()
	defer cockpitIn.Close()
	defer cockpitOut.Close()

	ch := New()
	runDone := make(chan error, 1)
	go func() {
		runDone <- ch.Run(cockpitIn, bridgeConn)
	}()

	payload := []byte("bridge-relay-check")
	if _, err := cockpitOut.Write(payload); err != nil {
		t.Fatalf("cockpitOut.Write() returned error: %v", err)
	}

	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(bridgePeer, buf); err != nil {
		t.Fatalf("bridgePeer.ReadFull() returned error: %v", err)
	}
	if !bytes.Equal(buf, payload) {
		t.Fatalf("bridgePeer payload = %q, want %q", buf, payload)
	}

	if err := bridgeConn.Close(); err != nil {
		t.Fatalf("bridgeConn.Close() returned error: %v", err)
	}

	select {
	case err := <-runDone:
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, ErrClosed) {
			t.Fatalf("Run() returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after bridge close")
	}
}

func TestChannelDoesNotForwardAfterClose(t *testing.T) {
	ch := New()
	if err := ch.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if err := ch.Close(); err != nil {
		t.Fatalf("Close() should be idempotent: %v", err)
	}
}
