package resource

import (
	"testing"
	"time"
)

func TestNodeCanonicalModel(t *testing.T) {
	observed := time.Date(2026, time.September, 20, 10, 0, 0, 0, time.UTC)

	node := Node{
		Metadata: NodeMetadata{
			Name:       "node001",
			Kind:       "Node",
			Generation: 42,
			ObservedAt: observed,
			Source:     "scontrol",
		},
		Spec: NodeSpec{
			Address:   "10.0.0.10",
			Hostname:  "node001.example.com",
			CPULoad:   27,
			RealMemory: 256000,
			AllocMemory: 8192,
			FreeMemory: 247808,
		},
		Status: NodeStatus{
			State:      "IDLE",
			StateFlags: []string{"POWER_UP"},
			Reason:     "",
		},
	}

	if got := node.Identity(); got != "node001" {
		t.Fatalf("Identity() = %q, want %q", got, "node001")
	}
	if got := node.Metadata.Generation; got != 42 {
		t.Fatalf("generation = %d, want 42", got)
	}
	if got := node.Spec.Address; got != "10.0.0.10" {
		t.Fatalf("address = %q, want %q", got, "10.0.0.10")
	}
	if got := node.Status.State; got != "IDLE" {
		t.Fatalf("state = %q, want %q", got, "IDLE")
	}
	if len(node.Status.StateFlags) != 1 || node.Status.StateFlags[0] != "POWER_UP" {
		t.Fatalf("state flags = %#v, want [POWER_UP]", node.Status.StateFlags)
	}
	if node.Metadata.Kind != "Node" {
		t.Fatalf("kind = %q, want %q", node.Metadata.Kind, "Node")
	}
	if !node.Metadata.ObservedAt.Equal(observed) {
		t.Fatalf("observedAt = %v, want %v", node.Metadata.ObservedAt, observed)
	}
}

func TestNodeStateAndFlagsArePreserved(t *testing.T) {
	node := NewNode("node002", 7)
	node.Status.State = "MIXED"
	node.Status.StateFlags = []string{"DRAIN", "RESERVED"}
	node.Status.Reason = "maintenance"

	if got := node.Identity(); got != "node002" {
		t.Fatalf("Identity() = %q, want %q", got, "node002")
	}
	if len(node.Status.StateFlags) != 2 {
		t.Fatalf("state flags length = %d, want 2", len(node.Status.StateFlags))
	}
	if node.Status.StateFlags[0] != "DRAIN" || node.Status.StateFlags[1] != "RESERVED" {
		t.Fatalf("state flags = %#v, want [DRAIN RESERVED]", node.Status.StateFlags)
	}
	if node.Status.Reason != "maintenance" {
		t.Fatalf("reason = %q, want %q", node.Status.Reason, "maintenance")
	}
	if node.Metadata.Generation != 7 {
		t.Fatalf("generation = %d, want 7", node.Metadata.Generation)
	}
}

func TestNodeSnapshotMetadata(t *testing.T) {
	node := NewNode("node003", 9)
	if node.Metadata.Source != "cockpit-slurm" {
		t.Fatalf("source = %q, want %q", node.Metadata.Source, "cockpit-slurm")
	}
	if node.Metadata.Name != "node003" {
		t.Fatalf("metadata name = %q, want %q", node.Metadata.Name, "node003")
	}
	if node.Spec.NodeName != "node003" {
		t.Fatalf("spec name = %q, want %q", node.Spec.NodeName, "node003")
	}
}
