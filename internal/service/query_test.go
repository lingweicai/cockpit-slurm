package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
)

func TestNodeQueryServiceReturnsNodesSnapshot(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 7)})

	svc := NewNodeQueryService(cache)
	snapshot, err := svc.Query(context.Background(), "nodes")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if snapshot.Resource != "nodes" {
		t.Fatalf("snapshot resource = %q, want %q", snapshot.Resource, "nodes")
	}
	if snapshot.Generation != 1 {
		t.Fatalf("snapshot generation = %d, want 1", snapshot.Generation)
	}
	if len(snapshot.Nodes) != 1 || snapshot.Nodes[0].Identity() != "node001" {
		t.Fatalf("snapshot nodes = %#v, want node001", snapshot.Nodes)
	}
}

func TestQueryHandlerReturnsSnapshotResponse(t *testing.T) {
	cache := resource.NewNodeCache()
	cache.ReplaceSnapshot([]resource.Node{resource.NewNode("node001", 7)})

	handler := NewQueryHandler(NewNodeQueryService(cache))
	msg := protocol.NewEnvelope("MSG-Q-1", protocol.MessageQuery, json.RawMessage(`{"resource":"nodes"}`))
	resp := handler.Handle(context.Background(), msg)
	if resp.Type != protocol.MessageQueryResponse {
		t.Fatalf("Type = %q, want %q", resp.Type, protocol.MessageQueryResponse)
	}
	if resp.MessageID != msg.MessageID {
		t.Fatalf("MessageID = %q, want %q", resp.MessageID, msg.MessageID)
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Payload, &payload); err != nil {
		t.Fatalf("decode response payload: %v", err)
	}
	if payload["resource"] != "nodes" {
		t.Fatalf("payload resource = %v, want %q", payload["resource"], "nodes")
	}
	if payload["generation"] == nil {
		t.Fatal("generation is missing from response")
	}
}
