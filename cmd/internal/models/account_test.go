package models

import "testing"

func TestAccountEqual(t *testing.T) {
	id := int32(7)
	direct := true

	left := &Account{
		Name:         "alpha",
		Description:  "Alpha",
		Organization: "org-a",
		Flags:        []string{"b", "a"},
		Associations: []AccountAssocShort{{Account: "acct", Cluster: "cluster", Partition: "part", User: "user", ID: &id}},
		Coordinators: []AccountCoordinator{{Name: "coord", Direct: &direct}},
	}
	right := &Account{
		Name:         "alpha",
		Description:  "Alpha",
		Organization: "org-a",
		Flags:        []string{"a", "b"},
		Associations: []AccountAssocShort{{Account: "acct", Cluster: "cluster", Partition: "part", User: "user", ID: &id}},
		Coordinators: []AccountCoordinator{{Name: "coord", Direct: &direct}},
	}

	if !left.Equal(right) {
		t.Fatal("Equal() = false, want true")
	}
}
