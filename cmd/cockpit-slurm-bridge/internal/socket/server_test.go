package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	bridgeaccount "github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/account"
	"github.com/lingweicai/cockpit-slurm/cmd/internal/models"
)

type socketStubProvider struct {
	accounts []*models.Account
}

func (s socketStubProvider) ListAccounts(ctx context.Context) ([]*models.Account, error) {
	return s.accounts, nil
}

func TestHandleConnectionEmitsReadyAndSnapshot(t *testing.T) {
	mgr := bridgeaccount.NewManager(socketStubProvider{
		accounts: []*models.Account{{Name: "alpha", Description: "Alpha", Organization: "org-a"}},
	})
	if _, err := mgr.LoadInitialCache(context.Background()); err != nil {
		t.Fatalf("LoadInitialCache() error = %v", err)
	}

	server := NewServer("", mgr)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		server.handleConnection(context.Background(), serverConn)
	}()

	reader := bufio.NewReader(clientConn)
	ready := readResponseLine(t, reader)
	if ready.Type != "connection.ready" {
		t.Fatalf("ready.Type = %q, want connection.ready", ready.Type)
	}
	if ready.ConnectionID == "" {
		t.Fatal("ready.ConnectionID is empty")
	}

	snapshot := readResponseLine(t, reader)
	if snapshot.Type != "snapshot" {
		t.Fatalf("snapshot.Type = %q, want snapshot", snapshot.Type)
	}
	if snapshot.Entity != "account" {
		t.Fatalf("snapshot.Entity = %q, want account", snapshot.Entity)
	}
	if snapshot.Generation != 1 {
		t.Fatalf("snapshot.Generation = %d, want 1", snapshot.Generation)
	}
	if len(snapshot.Items) != 1 || snapshot.Items[0].Name != "alpha" {
		t.Fatalf("snapshot.Items = %#v", snapshot.Items)
	}

	if _, err := clientConn.Write([]byte(`{"request_id":"req-1","type":"list","entity":"account"}` + "\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	listResp := readResponseLine(t, reader)
	if listResp.RequestID != "req-1" {
		t.Fatalf("listResp.RequestID = %q, want req-1", listResp.RequestID)
	}
	if listResp.Type != "snapshot" {
		t.Fatalf("listResp.Type = %q, want snapshot", listResp.Type)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].Name != "alpha" {
		t.Fatalf("listResp.Items = %#v", listResp.Items)
	}

	clientConn.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handleConnection to exit")
	}
}

func readResponseLine(t *testing.T, reader *bufio.Reader) response {
	t.Helper()

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("ReadString() error = %v", err)
	}

	var resp response
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v, payload=%q", err, line)
	}
	return resp
}
