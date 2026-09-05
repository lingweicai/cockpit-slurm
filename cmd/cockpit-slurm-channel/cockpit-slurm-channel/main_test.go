package main

import (
	"strings"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/ipc"
)

func TestResolveSocketPathUsesDefaultWhenUnset(t *testing.T) {
	t.Setenv("COCKPIT_SLURM_BRIDGE_SOCKET_PATH", "")

	got, err := resolveSocketPath()
	if err != nil {
		t.Fatalf("resolveSocketPath() returned error: %v", err)
	}
	if got != ipc.DefaultSocketPath {
		t.Fatalf("resolveSocketPath() = %q, want %q", got, ipc.DefaultSocketPath)
	}
}

func TestResolveSocketPathRejectsRelativeOverride(t *testing.T) {
	t.Setenv("COCKPIT_SLURM_BRIDGE_SOCKET_PATH", "relative/bridge.sock")

	_, err := resolveSocketPath()
	if err == nil {
		t.Fatal("resolveSocketPath() should reject a relative socket path")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("resolveSocketPath() error = %q, want to mention absolute path", err)
	}
}
