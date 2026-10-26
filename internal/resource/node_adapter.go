package resource

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	var shape map[string]json.RawMessage
	if err := json.Unmarshal(out, &shape); err != nil {
		return nil, fmt.Errorf("decode scontrol nodes JSON: %w", err)
	}
	rawNodes, exists := shape["nodes"]
	if !exists {
		return nil, errors.New(`decode scontrol nodes JSON: missing "nodes" field`)
	}
	trimmed := bytes.TrimSpace(rawNodes)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, errors.New(`decode scontrol nodes JSON: "nodes" must be an array`)
	}

	resp := models.V0043OpenapiNodesResp{}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode scontrol nodes JSON: %w", err)
	}
	if resp.Errors != nil && len(*resp.Errors) > 0 {
		return nil, errors.New("scontrol nodes response contains errors")
	}

	nodes := make([]Node, 0, len(resp.Nodes))
	for _, n := range resp.Nodes {
		nodes = append(nodes, MapSlurmNode(n))
	}
	return nodes, nil
}
