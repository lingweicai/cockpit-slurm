package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	models "github.com/lingweicai/cockpit-slurm/hack/openapi"
)

// NodeAdapter reads the node snapshot from Slurm and normalizes it to the canonical internal model.
type NodeAdapter interface {
	ListNodes(ctx context.Context) ([]Node, error)
}

type slurmNodeAdapter struct {
	execCommandContext func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func NewSlurmNodeAdapter() NodeAdapter {
	return &slurmNodeAdapter{
		execCommandContext: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			return cmd.Output()
		},
	}
}

func (a *slurmNodeAdapter) ListNodes(ctx context.Context) ([]Node, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.execCommandContext == nil {
		a.execCommandContext = NewSlurmNodeAdapter().(*slurmNodeAdapter).execCommandContext
	}
	out, err := a.execCommandContext(ctx, "scontrol", "show", "nodes", "--json")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("scontrol show nodes --json failed: exit=%d stderr=%s", exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("invoke scontrol show nodes --json: %w", err)
	}

	resp := models.V0043OpenapiNodesResp{}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode scontrol nodes JSON: %w", err)
	}

	nodes := make([]Node, 0, len(resp.Nodes))
	for _, n := range resp.Nodes {
		nodes = append(nodes, MapSlurmNode(n))
	}
	return nodes, nil
}
