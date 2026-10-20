package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	models "github.com/lingweicai/cockpit-slurm/hack/openapi"
)

const (
	sacctmgrPath          = "/usr/bin/sacctmgr"
	sacctmgrLookupTimeout = 5 * time.Second
)

type sacctmgrUserLookup struct {
	execCommandContext func(ctx context.Context, name string, args ...string) ([]byte, error)
}

func NewSacctmgrUserLookup() SlurmUserLookup {
	return &sacctmgrUserLookup{execCommandContext: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, name, args...).Output()
	}}
}

func (l *sacctmgrUserLookup) LookupUser(ctx context.Context, username string) (SlurmUser, bool, error) {
	if !validSlurmUsername(username) {
		return SlurmUser{}, false, fmt.Errorf("%w: invalid username", ErrSlurmUserLookup)
	}
	if l == nil || l.execCommandContext == nil {
		return SlurmUser{}, false, fmt.Errorf("%w: sacctmgr executor is unavailable", ErrSlurmUserLookup)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(ctx, sacctmgrLookupTimeout)
	defer cancel()

	output, err := l.execCommandContext(lookupCtx, sacctmgrPath,
		"--readonly", "--json", "show", "user", "where", "name="+username, "withcoords", "withassoc")
	if err != nil {
		return SlurmUser{}, false, fmt.Errorf("%w: sacctmgr query failed", ErrSlurmUserLookup)
	}
	return parseSacctmgrUser(output, username)
}

func parseSacctmgrUser(output []byte, requestedUsername string) (SlurmUser, bool, error) {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(output, &shape); err != nil {
		return SlurmUser{}, false, fmt.Errorf("%w: invalid sacctmgr JSON", ErrSlurmUserLookup)
	}
	rawUsers, ok := shape["users"]
	if !ok {
		return SlurmUser{}, false, fmt.Errorf("%w: sacctmgr response has no users field", ErrSlurmUserLookup)
	}
	if trimmed := bytes.TrimSpace(rawUsers); len(trimmed) == 0 || trimmed[0] != '[' {
		return SlurmUser{}, false, fmt.Errorf("%w: sacctmgr users field is not an array", ErrSlurmUserLookup)
	}

	response := models.V0043OpenapiUsersResp{}
	if err := json.Unmarshal(output, &response); err != nil {
		return SlurmUser{}, false, fmt.Errorf("%w: invalid sacctmgr user record", ErrSlurmUserLookup)
	}
	if response.Errors != nil && len(*response.Errors) > 0 {
		return SlurmUser{}, false, fmt.Errorf("%w: sacctmgr returned an error", ErrSlurmUserLookup)
	}
	if len(response.Users) == 0 {
		return SlurmUser{}, false, nil
	}
	if len(response.Users) != 1 || response.Users[0].Name != requestedUsername {
		return SlurmUser{}, false, fmt.Errorf("%w: sacctmgr returned an unexpected user record", ErrSlurmUserLookup)
	}

	user := response.Users[0]
	adminLevel := ""
	if user.AdministratorLevel != nil && len(*user.AdministratorLevel) == 1 {
		adminLevel = string((*user.AdministratorLevel)[0])
	} else if user.AdministratorLevel != nil && len(*user.AdministratorLevel) > 1 {
		return SlurmUser{}, false, fmt.Errorf("%w: sacctmgr returned multiple AdminLevel values", ErrSlurmUserLookup)
	}

	accounts := make([]string, 0)
	if user.Associations != nil {
		seen := make(map[string]struct{})
		for _, association := range *user.Associations {
			if association.Account == nil || *association.Account == "" {
				continue
			}
			if _, exists := seen[*association.Account]; exists {
				continue
			}
			seen[*association.Account] = struct{}{}
			accounts = append(accounts, *association.Account)
		}
	}
	sort.Strings(accounts)

	coordinators := make([]string, 0)
	if user.Coordinators != nil {
		for _, coordinator := range *user.Coordinators {
			if coordinator.Name != "" {
				coordinators = append(coordinators, coordinator.Name)
			}
		}
	}
	sort.Strings(coordinators)
	coordinators = compactStrings(coordinators)

	return SlurmUser{
		Username:            user.Name,
		AdminLevel:          adminLevel,
		Accounts:            accounts,
		CoordinatorAccounts: coordinators,
	}, true, nil
}

func validSlurmUsername(username string) bool {
	if username == "" || len(username) > 255 {
		return false
	}
	for _, char := range username {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("_.-", char)) {
			return false
		}
	}
	return true
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

var _ SlurmUserLookup = (*sacctmgrUserLookup)(nil)
