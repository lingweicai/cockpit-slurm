package account

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	bridgeentity "github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/entity"
	"github.com/lingweicai/cockpit-slurm/cmd/internal/models"
	"github.com/lingweicai/cockpit-slurm/cmd/internal/provider"
)

// CommandRunner executes a command and returns its combined output.
type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Manager loads, refreshes, and mutates account state for the bridge.
type Manager struct {
	entityManager *bridgeentity.EntityManager[models.Account]
	runner        CommandRunner
	broadcaster   *Broadcaster
	requests      *RequestCache
	entity        models.AccountEntity
}

// NewManager creates an account manager backed by the given provider.
func NewManager(p provider.AccountProvider) *Manager {
	return NewManagerWithRunner(p, nil)
}

// NewManagerWithRunner creates an account manager with a custom command runner.
func NewManagerWithRunner(p provider.AccountProvider, runner CommandRunner) *Manager {
	if runner == nil {
		runner = defaultCommandRunner
	}

	entityManager := bridgeentity.NewEntityManager(
		"account",
		accountProviderAdapter{provider: p},
		accountKey,
		cloneAccountForEntityManager,
		func(a, b *models.Account) bool { return a.Equal(b) },
	)

	return &Manager{
		entityManager: entityManager,
		runner:        runner,
		broadcaster:   NewBroadcaster(),
		requests:      NewRequestCache(5 * time.Minute),
		entity:        models.NewAccountEntity(),
	}
}

// Cache returns the backing account cache.
func (m *Manager) Cache() *bridgeentity.EntityCache[models.Account] {
	if m == nil {
		return nil
	}
	return m.entityManager.Cache()
}

// Broadcaster returns the event broadcaster.
func (m *Manager) Broadcaster() *Broadcaster {
	if m == nil {
		return nil
	}
	return m.broadcaster
}

// EventChannel subscribes to account update events.
func (m *Manager) EventChannel() chan Event {
	if m == nil || m.broadcaster == nil {
		return nil
	}
	return m.broadcaster.Subscribe()
}

// LoadInitialCache fetches all accounts from Slurm and populates the cache.
func (m *Manager) LoadInitialCache(ctx context.Context) (int, error) {
	if m == nil {
		return 0, fmt.Errorf("account manager is nil")
	}
	return m.entityManager.LoadInitialCache(ctx)
}

// Snapshot returns the current cache contents and generation metadata.
func (m *Manager) Snapshot() ([]*models.Account, uint64) {
	if m == nil || m.entityManager == nil {
		return nil, 0
	}
	accounts, generation := m.entityManager.Snapshot()
	return accounts, generation
}

// Run polls Slurm periodically and refreshes the cache.
func (m *Manager) Run(ctx context.Context, interval time.Duration) error {
	if m == nil {
		return fmt.Errorf("account manager is nil")
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return m.entityManager.Run(ctx, interval)
}

// RefreshOnce reloads accounts once and broadcasts any resulting diff.
func (m *Manager) RefreshOnce(ctx context.Context) (AccountDiff, error) {
	if m == nil {
		return AccountDiff{}, fmt.Errorf("account manager is nil")
	}

	genericDiff, err := m.entityManager.RefreshOnce(ctx)
	if err != nil {
		return AccountDiff{}, err
	}
	diff := AccountDiff(genericDiff)
	if len(diff.Added) == 0 && len(diff.Modified) == 0 && len(diff.Deleted) == 0 {
		return diff, nil
	}

	generation := m.entityManager.Cache().Generation()
	m.broadcastAccountEvent("account.updated", bridgeentity.EntityEvent[models.Account]{
		Entity:     "account",
		Generation: generation,
		Added:      diff.Added,
		Modified:   diff.Modified,
		Deleted:    diff.Deleted,
	})
	return diff, nil
}

// AddAccount executes sacctmgr add account and confirms the final state.
func (m *Manager) AddAccount(ctx context.Context, requestID string, spec models.AccountCreateSpec) (RequestResult, error) {
	return m.executeIdempotentRequest(requestID, func() (RequestResult, error) {
		args, err := m.entity.AddCommand(spec)
		if err != nil {
			return m.errorResult(requestID, err), err
		}
		if _, err := m.run(ctx, args...); err != nil {
			return m.errorResult(requestID, fmt.Errorf("execute %s: %w", joinCommand(args), err)), err
		}

		confirmed, err := m.fetchAccount(ctx, spec.Name)
		if err != nil {
			return m.errorResult(requestID, err), err
		}

		m.entityManager.Cache().Set(confirmed)
		generation := m.entityManager.Cache().Generation()
		event := bridgeentity.EntityEvent[models.Account]{
			Entity:     "account",
			Generation: generation,
			Added:      []*models.Account{confirmed},
		}
		m.broadcastAccountEvent("account.updated", event)

		return RequestResult{
			RequestID:  requestID,
			Status:     "ok",
			Data:       confirmed,
			Generation: generation,
		}, nil
	})
}

// ModifyAccount executes sacctmgr modify account and confirms the final state.
func (m *Manager) ModifyAccount(ctx context.Context, requestID, accountName string, spec models.AccountUpdateSpec) (RequestResult, error) {
	return m.executeIdempotentRequest(requestID, func() (RequestResult, error) {
		args, err := m.entity.ModifyCommand(accountName, spec)
		if err != nil {
			return m.errorResult(requestID, err), err
		}
		if _, err := m.run(ctx, args...); err != nil {
			return m.errorResult(requestID, fmt.Errorf("execute %s: %w", joinCommand(args), err)), err
		}

		confirmed, err := m.fetchAccount(ctx, accountName)
		if err != nil {
			return m.errorResult(requestID, err), err
		}

		m.entityManager.Cache().Set(confirmed)
		generation := m.entityManager.Cache().Generation()
		event := bridgeentity.EntityEvent[models.Account]{
			Entity:     "account",
			Generation: generation,
			Modified:   []*models.Account{confirmed},
		}
		m.broadcastAccountEvent("account.updated", event)

		return RequestResult{
			RequestID:  requestID,
			Status:     "ok",
			Data:       confirmed,
			Generation: generation,
		}, nil
	})
}

// DeleteAccount executes sacctmgr delete account and confirms the account is gone.
func (m *Manager) DeleteAccount(ctx context.Context, requestID, accountName string) (RequestResult, error) {
	return m.executeIdempotentRequest(requestID, func() (RequestResult, error) {
		existing, _ := m.entityManager.Cache().Get(accountName)
		args, err := m.entity.DeleteCommand(accountName)
		if err != nil {
			return m.errorResult(requestID, err), err
		}
		if _, err := m.run(ctx, args...); err != nil {
			return m.errorResult(requestID, fmt.Errorf("execute %s: %w", joinCommand(args), err)), err
		}

		confirmed, found, err := m.fetchAccountMaybe(ctx, accountName)
		if err != nil {
			return m.errorResult(requestID, err), err
		}
		if found {
			return m.errorResult(requestID, fmt.Errorf("account %q still exists after delete", accountName)), fmt.Errorf("account %q still exists after delete", accountName)
		}

		if existing == nil {
			existing = &models.Account{Name: accountName}
		}
		m.entityManager.Cache().Delete(accountName)
		generation := m.entityManager.Cache().Generation()
		m.broadcastAccountEvent("account.updated", bridgeentity.EntityEvent[models.Account]{
			Entity:     "account",
			Generation: generation,
			Deleted:    []*models.Account{existing},
		})

		return RequestResult{
			RequestID:  requestID,
			Status:     "ok",
			Data:       confirmed,
			Generation: generation,
		}, nil
	})
}

func (m *Manager) executeIdempotentRequest(requestID string, fn func() (RequestResult, error)) (RequestResult, error) {
	if m == nil || requestID == "" || m.requests == nil {
		return fn()
	}

	if cached, ok, wait := m.requests.Begin(requestID); ok {
		return cached, nil
	} else if wait != nil {
		<-wait
		if cached, ok := m.requests.Get(requestID); ok {
			return cached, nil
		}
		return RequestResult{}, fmt.Errorf("request %q completed without cached result", requestID)
	}

	result, err := fn()
	if err != nil {
		if result.RequestID == "" {
			result.RequestID = requestID
		}
		result.Status = "error"
		result.Error = err.Error()
		result.Timestamp = time.Now().UTC()
		m.requests.Complete(requestID, result)
		return result, err
	}

	if result.RequestID == "" {
		result.RequestID = requestID
	}
	result.Timestamp = time.Now().UTC()
	m.requests.Complete(requestID, result)
	return result, nil
}

func (m *Manager) fetchAccount(ctx context.Context, accountName string) (*models.Account, error) {
	account, found, err := m.fetchAccountMaybe(ctx, accountName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("account %q not found after refresh", accountName)
	}
	return account, nil
}

func (m *Manager) fetchAccountMaybe(ctx context.Context, accountName string) (*models.Account, bool, error) {
	args, err := m.entity.GetCommand(accountName)
	if err != nil {
		return nil, false, err
	}

	raw, err := m.run(ctx, args...)
	if err != nil {
		return nil, false, fmt.Errorf("execute %s: %w", joinCommand(args), err)
	}

	resp, err := m.entity.ParseListJSON(raw)
	if err != nil {
		return nil, false, err
	}
	if len(resp.Errors) > 0 {
		return nil, false, fmt.Errorf("sacctmgr list account returned %d error(s)", len(resp.Errors))
	}
	if len(resp.Accounts) == 0 {
		return nil, false, nil
	}

	account := models.AccountFromV0043Account(resp.Accounts[0])
	return &account, true, nil
}

func (m *Manager) run(ctx context.Context, args ...string) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("account manager is nil")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	runner := m.runner
	if runner == nil {
		runner = defaultCommandRunner
	}
	return runner(ctx, args[0], args[1:]...)
}

func (m *Manager) broadcastAccountEvent(eventType string, payload bridgeentity.EntityEvent[models.Account]) {
	if m == nil || m.broadcaster == nil {
		return
	}
	m.broadcaster.Broadcast(Event{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Data:      payload,
	})
}

func accountKey(account *models.Account) string {
	if account == nil {
		return ""
	}
	return account.Name
}

func cloneAccountForEntityManager(account *models.Account) *models.Account {
	if account == nil {
		return nil
	}

	cloned := *account
	cloned.Flags = append([]string(nil), account.Flags...)
	cloned.Associations = append([]models.AccountAssocShort(nil), account.Associations...)
	cloned.Coordinators = append([]models.AccountCoordinator(nil), account.Coordinators...)
	return &cloned
}

type accountProviderAdapter struct {
	provider provider.AccountProvider
}

func (a accountProviderAdapter) List(ctx context.Context) ([]*models.Account, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("account provider is nil")
	}
	return a.provider.ListAccounts(ctx)
}

func defaultCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, err, bytes.TrimSpace(raw))
	}
	return raw, nil
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

func (m *Manager) errorResult(requestID string, err error) RequestResult {
	return RequestResult{
		RequestID: requestID,
		Status:    "error",
		Error:     err.Error(),
		Timestamp: time.Now().UTC(),
	}
}
