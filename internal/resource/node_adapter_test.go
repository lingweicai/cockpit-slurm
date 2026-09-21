package resource

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	models "github.com/lingweicai/cockpit-slurm/hack/openapi"
)

func TestSlurmNodeAdapterMapsGeneratedNodes(t *testing.T) {
	payload := map[string]any{
		"nodes": []map[string]any{
			{
				"name":         "node001",
				"hostname":     "node001.example.com",
				"address":      "10.0.0.10",
				"cpu_load":     27,
				"real_memory":  256000,
				"alloc_memory": 8192,
				"free_mem": map[string]any{"number": 247808, "set": true},
				"reason":       "maintenance",
				"state":        []string{"IDLE", "POWER_UP"},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	adapter := &slurmNodeAdapter{execCommandContext: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "scontrol" {
			t.Fatalf("command = %q, want %q", name, "scontrol")
		}
		if strings.Join(args, " ") != "show nodes --json" {
			t.Fatalf("args = %q, want %q", strings.Join(args, " "), "show nodes --json")
		}
		return data, nil
	}}

	nodes, err := adapter.ListNodes(context.Background())
	if err != nil {
		t.Fatalf("ListNodes returned error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if nodes[0].Identity() != "node001" {
		t.Fatalf("Identity() = %q, want %q", nodes[0].Identity(), "node001")
	}
	if nodes[0].Status.State != "IDLE" {
		t.Fatalf("state = %q, want %q", nodes[0].Status.State, "IDLE")
	}
	if len(nodes[0].Status.StateFlags) != 1 || nodes[0].Status.StateFlags[0] != "POWER_UP" {
		t.Fatalf("state flags = %#v, want [POWER_UP]", nodes[0].Status.StateFlags)
	}
}

func TestSlurmNodeAdapterHandlesCommandFailure(t *testing.T) {
	adapter := &slurmNodeAdapter{execCommandContext: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, &exec.ExitError{ProcessState: nil, Stderr: []byte("boom\n")}
	}}

	_, err := adapter.ListNodes(context.Background())
	if err == nil || !strings.Contains(err.Error(), "scontrol show nodes --json failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeneratedNodeStateValues(t *testing.T) {
	stateValues := []models.V0043NodeState{models.V0043NodeStateIDLE, models.V0043NodeStatePOWERUP}
	if got := summarizeNodeState(stateValues); got != "IDLE" {
		t.Fatalf("summarizeNodeState = %q, want %q", got, "IDLE")
	}
}
