package resource

import (
	"slices"
	"sort"
)

type NodeChangeKind string

const (
	NodeAdded   NodeChangeKind = "added"
	NodeUpdated NodeChangeKind = "updated"
	NodeRemoved NodeChangeKind = "removed"
)

type NodeChange struct {
	Kind NodeChangeKind `json:"kind"`
	Node Node           `json:"node"`
}

type NodeChangeBatch struct {
	Resource   string       `json:"resource"`
	Event      string       `json:"event"`
	Generation uint64       `json:"generation"`
	Changes    []NodeChange `json:"changes"`
}

func sameNodeState(a, b Node) bool {
	return a.Spec == b.Spec &&
		a.Status.State == b.Status.State &&
		slices.Equal(a.Status.StateFlags, b.Status.StateFlags) &&
		a.Status.Reason == b.Status.Reason
}

func sortNodeChanges(changes []NodeChange) {
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Node.Identity() < changes[j].Node.Identity()
	})
}
