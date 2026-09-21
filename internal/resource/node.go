package resource

import (
	"time"

	models "github.com/lingweicai/cockpit-slurm/hack/openapi"
)

// Node is the canonical backend-owned representation of a Slurm node.
// It intentionally avoids Cockpit and React dependencies.
type Node struct {
	Metadata NodeMetadata `json:"metadata"`
	Spec     NodeSpec     `json:"spec"`
	Status   NodeStatus   `json:"status"`
}

func NewNode(name string, generation int64) Node {
	return Node{
		Metadata: NodeMetadata{
			Name:       name,
			Kind:       "Node",
			Generation: generation,
			ObservedAt: time.Now().UTC(),
			Source:     "cockpit-slurm",
		},
		Spec: NodeSpec{
			NodeName: name,
		},
		Status: NodeStatus{},
	}
}

func (n Node) Identity() string {
	if n.Metadata.Name != "" {
		return n.Metadata.Name
	}
	return n.Spec.NodeName
}

// NodeMetadata identifies a resource within the backend cache.
type NodeMetadata struct {
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	Generation int64     `json:"generation"`
	ObservedAt time.Time `json:"observedAt"`
	Source     string    `json:"source"`
}

// NodeSpec contains the stable node identity and essential static runtime data.
type NodeSpec struct {
	NodeName    string `json:"nodeName"`
	Address     string `json:"address,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	CPULoad     int32  `json:"cpuLoad,omitempty"`
	RealMemory  int64  `json:"realMemory,omitempty"`
	AllocMemory int64  `json:"allocMemory,omitempty"`
	FreeMemory  int64  `json:"freeMemory,omitempty"`
}

// NodeStatus preserves the Slurm state and any meaningful flags.
type NodeStatus struct {
	State      string   `json:"state,omitempty"`
	StateFlags []string `json:"stateFlags,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

// MapSlurmNode converts the generated Slurm OpenAPI node model into the canonical backend Node contract.
func MapSlurmNode(in models.V0043Node) Node {
	node := NewNode(nodeNameFromGenerated(in), 0)

	if in.Address != nil {
		node.Spec.Address = *in.Address
	}
	if in.Hostname != nil {
		node.Spec.Hostname = *in.Hostname
	}
	if in.Name != nil {
		node.Spec.NodeName = *in.Name
		node.Metadata.Name = *in.Name
	}
	if in.CpuLoad != nil {
		node.Spec.CPULoad = *in.CpuLoad
	}
	if in.RealMemory != nil {
		node.Spec.RealMemory = *in.RealMemory
	}
	if in.AllocMemory != nil {
		node.Spec.AllocMemory = *in.AllocMemory
	}
	if in.FreeMem != nil && in.FreeMem.Number != nil && in.FreeMem.Set != nil && *in.FreeMem.Set {
		node.Spec.FreeMemory = *in.FreeMem.Number
	}
	if in.Reason != nil {
		node.Status.Reason = *in.Reason
	}
	if in.State != nil {
		node.Status.State = summarizeNodeState(*in.State)
		node.Status.StateFlags = stateFlagsFromGenerated(*in.State)
	}
	return node
}

func nodeNameFromGenerated(in models.V0043Node) string {
	if in.Name != nil && *in.Name != "" {
		return *in.Name
	}
	if in.Hostname != nil && *in.Hostname != "" {
		return *in.Hostname
	}
	if in.Address != nil && *in.Address != "" {
		return *in.Address
	}
	return ""
}

func summarizeNodeState(states []models.V0043NodeState) string {
	if len(states) == 0 {
		return ""
	}

	primary := 0
	for _, state := range states {
		switch state {
		case models.V0043NodeStateALLOCATED,
			models.V0043NodeStateDOWN,
			models.V0043NodeStateERROR,
			models.V0043NodeStateFAIL,
			models.V0043NodeStateFUTURE,
			models.V0043NodeStateIDLE,
			models.V0043NodeStateMIXED,
			models.V0043NodeStatePLANNED,
			models.V0043NodeStateUNKNOWN:
			primary++
		}
	}
	if primary > 1 {
		return string(models.V0043NodeStateMIXED)
	}
	for _, state := range states {
		switch state {
		case models.V0043NodeStateALLOCATED,
			models.V0043NodeStateDOWN,
			models.V0043NodeStateERROR,
			models.V0043NodeStateFAIL,
			models.V0043NodeStateFUTURE,
			models.V0043NodeStateIDLE,
			models.V0043NodeStateMIXED,
			models.V0043NodeStatePLANNED,
			models.V0043NodeStateUNKNOWN:
			return string(state)
		}
	}
	return string(states[0])
}

func stateFlagsFromGenerated(states []models.V0043NodeState) []string {
	flags := make([]string, 0, len(states))
	for _, state := range states {
		switch state {
		case models.V0043NodeStateALLOCATED,
			models.V0043NodeStateDOWN,
			models.V0043NodeStateERROR,
			models.V0043NodeStateFAIL,
			models.V0043NodeStateFUTURE,
			models.V0043NodeStateIDLE,
			models.V0043NodeStateMIXED,
			models.V0043NodeStatePLANNED,
			models.V0043NodeStateUNKNOWN:
			continue
		default:
			flags = append(flags, string(state))
		}
	}
	return flags
}
