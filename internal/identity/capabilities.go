package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
)

const (
	CapabilityNodeDrain  = "nodes.drain"
	CapabilityNodeResume = "nodes.resume"
)

var (
	ErrIdentityUnavailable = errors.New("requesting identity unavailable")
	ErrSlurmUserNotFound   = errors.New("Slurm user not found")
	ErrSlurmUserLookup     = errors.New("Slurm user lookup failed")
	ErrUnknownAdminLevel   = errors.New("unknown Slurm AdminLevel")
)

type SlurmUser struct {
	Username            string
	AdminLevel          string
	Accounts            []string
	CoordinatorAccounts []string
}

type SlurmUserLookup interface {
	LookupUser(ctx context.Context, username string) (SlurmUser, bool, error)
}

type CapabilityResolver struct {
	users SlurmUserLookup
}

func NewCapabilityResolver(users SlurmUserLookup) *CapabilityResolver {
	return &CapabilityResolver{users: users}
}

func (r *CapabilityResolver) Resolve(ctx context.Context, requester connection.UserContext) (connection.UserContext, error) {
	if requester.Username == "" {
		return connection.UserContext{}, ErrIdentityUnavailable
	}
	if r == nil || r.users == nil {
		return connection.UserContext{}, fmt.Errorf("%w: resolver is not initialized", ErrSlurmUserLookup)
	}

	user, found, err := r.users.LookupUser(ctx, requester.Username)
	if err != nil {
		return connection.UserContext{}, fmt.Errorf("%w: %v", ErrSlurmUserLookup, err)
	}
	if !found {
		return connection.UserContext{}, ErrSlurmUserNotFound
	}
	if user.Username == "" || user.Username != requester.Username {
		return connection.UserContext{}, fmt.Errorf("%w: Slurm username does not match requester", ErrIdentityUnavailable)
	}

	adminLevel, err := normalizeAdminLevel(user.AdminLevel)
	if err != nil {
		return connection.UserContext{}, err
	}

	resolved := requester
	resolved.AdminLevel = adminLevel
	resolved.Accounts = append([]string(nil), user.Accounts...)
	resolved.CoordinatorAccounts = append([]string(nil), user.CoordinatorAccounts...)
	allowed := adminLevel == "admin"
	resolved.Capabilities = map[string]bool{
		CapabilityNodeDrain:  allowed,
		CapabilityNodeResume: allowed,
	}
	return resolved, nil
}

func normalizeAdminLevel(adminLevel string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(adminLevel)) {
	case "none":
		return "none", nil
	case "operator":
		return "operator", nil
	case "admin", "administrator":
		return "admin", nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownAdminLevel, adminLevel)
	}
}
