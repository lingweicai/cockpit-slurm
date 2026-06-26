package provider

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/lingweicai/cockpit-slurm/cmd/internal/models"
)

// CommandRunner executes a command and returns its combined output.
type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// SacctmgrAccountProvider loads account data from `sacctmgr --json`.
type SacctmgrAccountProvider struct {
	run CommandRunner
}

// NewSacctmgrAccountProvider creates a provider that executes the sacctmgr CLI.
func NewSacctmgrAccountProvider() *SacctmgrAccountProvider {
	return &SacctmgrAccountProvider{run: runCommand}
}

// NewSacctmgrAccountProviderWithRunner creates a provider with a custom command runner.
func NewSacctmgrAccountProviderWithRunner(run CommandRunner) *SacctmgrAccountProvider {
	return &SacctmgrAccountProvider{run: run}
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// ListAccounts returns all Slurm accounts parsed from sacctmgr JSON output.
func (p *SacctmgrAccountProvider) ListAccounts(ctx context.Context) ([]*models.Account, error) {
	if p == nil {
		return nil, fmt.Errorf("sacctmgr account provider is nil")
	}
	runner := p.run
	if runner == nil {
		runner = runCommand
	}

	entity := models.NewAccountEntity()
	cmdArgs := entity.ListCommand()

	raw, err := runner(ctx, cmdArgs[0], cmdArgs[1:]...)
	if err != nil {
		return nil, fmt.Errorf("execute %s: %w", joinCommand(cmdArgs), err)
	}

	resp, err := entity.ParseListJSON(raw)
	if err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("sacctmgr list account returned %d error(s)", len(resp.Errors))
	}

	accounts := make([]*models.Account, 0, len(resp.Accounts))
	for i := range resp.Accounts {
		account := resp.Accounts[i]
		accounts = append(accounts, &account)
	}

	return accounts, nil
}

func joinCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}

	out := args[0]
	for _, arg := range args[1:] {
		out += " " + arg
	}
	return out
}
