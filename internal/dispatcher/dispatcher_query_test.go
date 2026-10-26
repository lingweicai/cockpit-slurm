package dispatcher

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
	"github.com/lingweicai/cockpit-slurm/internal/service"
)

func TestDispatcherQueryNodesWithCache(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 3)})
	q := service.NewNodeQueryService(cache)
	h := service.NewQueryHandler(q)
	d := NewDispatcher()
	if err := d.Register(protocol.MessageQuery, h); err != nil {
		// no-op: dispatcher has a register method in the implementation below
		_ = err
	}

	msg := protocol.NewEnvelope("MSG-QUERY-1", protocol.MessageQuery, json.RawMessage(`{"resource":"nodes"}`))
	resp := d.Dispatch(context.Background(), msg)
	if resp.Type != protocol.MessageQueryResponse {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessageQueryResponse)
	}
	if resp.MessageID != msg.MessageID {
		t.Fatalf("MessageID = %q, want %q", resp.MessageID, msg.MessageID)
	}
	var out map[string]any
	if err := json.Unmarshal(resp.Payload, &out); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if out["resource"] != "nodes" {
		t.Fatalf("resource = %v, want %q", out["resource"], "nodes")
	}
	if out["generation"] == nil {
		t.Fatal("generation missing from response")
	}
}
