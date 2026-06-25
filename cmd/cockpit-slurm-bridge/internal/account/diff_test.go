package account

import (
	"testing"

	"github.com/lingweicai/cockpit-slurm/cmd/internal/models"
)

func TestCompareAccounts(t *testing.T) {
	oldMap := map[string]*models.Account{
		"alpha": accountWithSlices("alpha", "Alpha", "org-a", []string{"b", "a"}),
		"beta":  accountWithSlices("beta", "Beta", "org-b", []string{"x"}),
	}
	newMap := map[string]*models.Account{
		"alpha": accountWithSlices("alpha", "Alpha updated", "org-a", []string{"a", "b"}),
		"gamma": accountWithSlices("gamma", "Gamma", "org-g", []string{"y"}),
	}

	diff := Compare(oldMap, newMap)
	if len(diff.Added) != 1 || diff.Added[0].Name != "gamma" {
		t.Fatalf("Added = %#v", diff.Added)
	}
	if len(diff.Modified) != 1 || diff.Modified[0].Name != "alpha" {
		t.Fatalf("Modified = %#v", diff.Modified)
	}
	if len(diff.Deleted) != 1 || diff.Deleted[0].Name != "beta" {
		t.Fatalf("Deleted = %#v", diff.Deleted)
	}
}

func TestCompareAccountsIgnoresSliceOrdering(t *testing.T) {
	left := accountWithSlices("alpha", "Alpha", "org-a", []string{"b", "a"})
	right := accountWithSlices("alpha", "Alpha", "org-a", []string{"a", "b"})

	diff := Compare(map[string]*models.Account{"alpha": left}, map[string]*models.Account{"alpha": right})
	if len(diff.Added) != 0 || len(diff.Modified) != 0 || len(diff.Deleted) != 0 {
		t.Fatalf("diff = %#v, want no changes", diff)
	}
}

func TestCompareAccountsNilMaps(t *testing.T) {
	diff := Compare(nil, nil)
	if len(diff.Added) != 0 || len(diff.Modified) != 0 || len(diff.Deleted) != 0 {
		t.Fatalf("diff = %#v, want empty", diff)
	}
}

func accountWithSlices(name, description, organization string, flags []string) *models.Account {
	id := int32(7)
	direct := true
	return &models.Account{
		Name:         name,
		Description:  description,
		Organization: organization,
		Flags:        flags,
		Associations: []models.AccountAssocShort{
			{
				Account:   "acct-a",
				Cluster:   "cluster-a",
				Partition: "partition-a",
				User:      "user-a",
				ID:        &id,
			},
		},
		Coordinators: []models.AccountCoordinator{
			{
				Name:   "coord-a",
				Direct: &direct,
			},
		},
	}
}
