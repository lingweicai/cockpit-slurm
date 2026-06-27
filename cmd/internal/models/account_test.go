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

func TestAccountOpenAPIRoundTrip(t *testing.T) {
	id := int32(7)
	direct := true
	original := Account{
		Name:         "alpha",
		Description:  "Alpha",
		Organization: "org-a",
		Flags:        []string{"b", "a"},
		Associations: []AccountAssocShort{{Account: "acct", Cluster: "cluster", Partition: "part", User: "user", ID: &id}},
		Coordinators: []AccountCoordinator{{Name: "coord", Direct: &direct}},
	}

	generated := original.ToV0043Account()
	if generated.Name != original.Name || generated.Description != original.Description || generated.Organization != original.Organization {
		t.Fatalf("generated = %#v, want matching scalar fields", generated)
	}
	if generated.Flags == nil || len(*generated.Flags) != 2 {
		t.Fatalf("generated.Flags = %#v, want 2 values", generated.Flags)
	}
	if generated.Associations == nil || len(*generated.Associations) != 1 {
		t.Fatalf("generated.Associations = %#v, want 1 value", generated.Associations)
	}
	if generated.Coordinators == nil || len(*generated.Coordinators) != 1 {
		t.Fatalf("generated.Coordinators = %#v, want 1 value", generated.Coordinators)
	}

	roundTrip := AccountFromV0043Account(generated)
	if !original.Equal(&roundTrip) {
		t.Fatalf("roundTrip = %#v, want %#v", roundTrip, original)
	}
}
