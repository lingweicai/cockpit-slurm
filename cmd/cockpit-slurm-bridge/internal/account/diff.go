package account

import (
	bridgeentity "github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/entity"
	"github.com/lingweicai/cockpit-slurm/cmd/internal/models"
)

// AccountDiff represents changes between two account snapshots.
type AccountDiff = bridgeentity.EntityDiff[models.Account]

// Compare computes the delta from oldMap to newMap.
func Compare(oldMap, newMap map[string]*models.Account) AccountDiff {
	return bridgeentity.Compare(oldMap, newMap, func(a, b *models.Account) bool {
		return a.Equal(b)
	}, cloneAccountForEntityManager, accountKey)
}
