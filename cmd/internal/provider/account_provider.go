package provider

import (
	"context"

	"github.com/lingweicai/cockpit-slurm/cmd/internal/models"
)

// AccountProvider retrieves Slurm account records.
type AccountProvider interface {
	ListAccounts(ctx context.Context) ([]*models.Account, error)
}
