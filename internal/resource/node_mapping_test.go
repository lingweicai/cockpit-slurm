package resource

import (
	"testing"

	models "github.com/lingweicai/cockpit-slurm/hack/openapi"
)

func TestMapSlurmNodeToCanonicalNode(t *testing.T) {
	stateValues := []models.V0043NodeState{
		models.V0043NodeStateIDLE,
		models.V0043NodeStatePOWERUP,
		models.V0043NodeStateDRAIN,
	}
	freeMem := &models.V0043Uint64NoValStruct{
		Number: ptrInt64(247808),
		Set:    ptrBool(true),
	}
	generated := models.V0043Node{
		Address:     ptrString("10.0.0.10"),
		AllocMemory: ptrInt64(8192),
		CpuLoad:    ptrInt32(27),
		FreeMem:     freeMem,
		Hostname:    ptrString("node001.example.com"),
		Name:        ptrString("node001"),
		RealMemory:  ptrInt64(256000),
		Reason:      ptrString("maintenance"),
		State:       &stateValues,
	}

	node := MapSlurmNode(generated)
	if node.Identity() != "node001" {
		t.Fatalf("Identity() = %q, want %q", node.Identity(), "node001")
	}
	if node.Spec.Address != "10.0.0.10" {
		t.Fatalf("address = %q, want %q", node.Spec.Address, "10.0.0.10")
	}
	if node.Spec.Hostname != "node001.example.com" {
		t.Fatalf("hostname = %q, want %q", node.Spec.Hostname, "node001.example.com")
	}
	if node.Spec.CPULoad != 27 {
		t.Fatalf("cpu load = %d, want 27", node.Spec.CPULoad)
	}
	if node.Spec.RealMemory != 256000 {
		t.Fatalf("real memory = %d, want 256000", node.Spec.RealMemory)
	}
	if node.Spec.AllocMemory != 8192 {
		t.Fatalf("alloc memory = %d, want 8192", node.Spec.AllocMemory)
	}
	if node.Spec.FreeMemory != 247808 {
		t.Fatalf("free memory = %d, want 247808", node.Spec.FreeMemory)
	}
	if node.Status.State != "IDLE" {
		t.Fatalf("state = %q, want %q", node.Status.State, "IDLE")
	}
	if len(node.Status.StateFlags) != 2 {
		t.Fatalf("state flags len = %d, want 2", len(node.Status.StateFlags))
	}
	if node.Status.StateFlags[0] != "POWER_UP" || node.Status.StateFlags[1] != "DRAIN" {
		t.Fatalf("state flags = %#v, want [POWER_UP DRAIN]", node.Status.StateFlags)
	}
	if node.Status.Reason != "maintenance" {
		t.Fatalf("reason = %q, want %q", node.Status.Reason, "maintenance")
	}
}

func ptrString(v string) *string { return &v }
func ptrInt32(v int32) *int32 { return &v }
func ptrInt64(v int64) *int64 { return &v }
func ptrBool(v bool) *bool { return &v }
