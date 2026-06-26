package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lingweicai/cockpit-slurm/cmd/cockpit-slurm-bridge/internal/entity"
	"github.com/lingweicai/cockpit-slurm/cmd/internal/models"
)

type stubProvider struct {
	accounts []*models.Account
	err      error
}

func (s stubProvider) ListAccounts(ctx context.Context) ([]*models.Account, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.accounts, nil
}

func TestManagerLoadInitialCache(t *testing.T) {
	mgr := NewManager(stubProvider{
		accounts: []*models.Account{
			{Name: "alpha", Description: "Alpha", Organization: "org-a"},
			{Name: "beta", Description: "Beta", Organization: "org-b"},
		},
	})

	count, err := mgr.LoadInitialCache(context.Background())
	if err != nil {
		t.Fatalf("LoadInitialCache() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("LoadInitialCache() count = %d, want 2", count)
	}

	cache := mgr.Cache()
	if cache == nil {
		t.Fatal("Cache() = nil, want cache")
	}
	if cache.Generation() != 1 {
		t.Fatalf("Generation() = %d, want 1", cache.Generation())
	}
	if cache.UpdatedAt().IsZero() {
		t.Fatal("UpdatedAt() is zero, want non-zero")
	}

	list := cache.List()
	if len(list) != 2 {
		t.Fatalf("List() length = %d, want 2", len(list))
	}
	if list[0].Name != "alpha" || list[1].Name != "beta" {
		t.Fatalf("List() = %#v", list)
	}
}

func TestManagerLoadInitialCacheError(t *testing.T) {
	mgr := NewManager(stubProvider{err: errors.New("sacctmgr failed")})

	count, err := mgr.LoadInitialCache(context.Background())
	if err == nil {
		t.Fatal("LoadInitialCache() error = nil, want non-nil")
	}
	if count != 0 {
		t.Fatalf("LoadInitialCache() count = %d, want 0", count)
	}
	if mgr.Cache().Generation() != 0 {
		t.Fatalf("Generation() = %d, want 0", mgr.Cache().Generation())
	}
}

func TestManagerRefreshOnceBroadcastsDiff(t *testing.T) {
	provider := &sequenceProvider{
		batches: [][]*models.Account{
			{
				{Name: "alpha", Description: "Alpha", Organization: "org-a"},
			},
			{
				{Name: "alpha", Description: "Alpha v2", Organization: "org-a"},
				{Name: "beta", Description: "Beta", Organization: "org-b"},
			},
		},
	}
	mgr := NewManager(provider)

	if _, err := mgr.LoadInitialCache(context.Background()); err != nil {
		t.Fatalf("LoadInitialCache() error = %v", err)
	}

	events := mgr.EventChannel()
	defer mgr.Broadcaster().Unsubscribe(events)

	diff, err := mgr.RefreshOnce(context.Background())
	if err != nil {
		t.Fatalf("RefreshOnce() error = %v", err)
	}
	if len(diff.Added) != 1 || diff.Added[0].Name != "beta" {
		t.Fatalf("Added = %#v", diff.Added)
	}
	if len(diff.Modified) != 1 || diff.Modified[0].Name != "alpha" {
		t.Fatalf("Modified = %#v", diff.Modified)
	}

	select {
	case evt := <-events:
		payload, ok := evt.Data.(entity.EntityEvent[models.Account])
		if !ok {
			t.Fatalf("event payload type = %T, want EntityEvent[models.Account]", evt.Data)
		}
		if payload.Generation != mgr.Cache().Generation() {
			t.Fatalf("generation = %d, want %d", payload.Generation, mgr.Cache().Generation())
		}
		if len(payload.Added) != 1 || len(payload.Modified) != 1 {
			t.Fatalf("payload = %#v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for refresh event")
	}
}

func TestManagerCrudRequestsUseIdempotencyAndConfirmation(t *testing.T) {
	state := newMutableAccountState()
	mgr := NewManager(state)
	mgr.runner = state.runner()

	addResult, err := mgr.AddAccount(context.Background(), "req-add", models.AccountCreateSpec{
		Name:         "alpha",
		Description:  "Alpha",
		Organization: "org-a",
	})
	if err != nil {
		t.Fatalf("AddAccount() error = %v", err)
	}
	if addResult.Status != "ok" {
		t.Fatalf("AddAccount() status = %q, want ok", addResult.Status)
	}
	if got := state.runCount("add"); got != 1 {
		t.Fatalf("add command count = %d, want 1", got)
	}

	dupResult, err := mgr.AddAccount(context.Background(), "req-add", models.AccountCreateSpec{
		Name:         "alpha",
		Description:  "Alpha",
		Organization: "org-a",
	})
	if err != nil {
		t.Fatalf("duplicate AddAccount() error = %v", err)
	}
	if dupResult.Status != "ok" {
		t.Fatalf("duplicate AddAccount() status = %q, want ok", dupResult.Status)
	}
	if got := state.runCount("add"); got != 1 {
		t.Fatalf("add command count after duplicate = %d, want 1", got)
	}

	modResult, err := mgr.ModifyAccount(context.Background(), "req-mod", "alpha", models.AccountUpdateSpec{
		Description: stringPtr("Alpha v2"),
	})
	if err != nil {
		t.Fatalf("ModifyAccount() error = %v", err)
	}
	if modResult.Status != "ok" {
		t.Fatalf("ModifyAccount() status = %q, want ok", modResult.Status)
	}

	delResult, err := mgr.DeleteAccount(context.Background(), "req-del", "alpha")
	if err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if delResult.Status != "ok" {
		t.Fatalf("DeleteAccount() status = %q, want ok", delResult.Status)
	}
	if _, ok := mgr.Cache().Get("alpha"); ok {
		t.Fatal("account still present after delete")
	}
}

type sequenceProvider struct {
	mu      sync.Mutex
	batches [][]*models.Account
	idx     int
}

func (s *sequenceProvider) ListAccounts(ctx context.Context) ([]*models.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.batches) == 0 {
		return nil, nil
	}
	if s.idx >= len(s.batches) {
		return cloneAccountsForTest(s.batches[len(s.batches)-1]), nil
	}
	batch := cloneAccountsForTest(s.batches[s.idx])
	s.idx++
	return batch, nil
}

type mutableAccountState struct {
	mu       sync.Mutex
	accounts map[string]models.Account
	runs     map[string]int
}

func (s *mutableAccountState) ListAccounts(ctx context.Context) ([]*models.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts := make([]*models.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		copy := account
		accounts = append(accounts, &copy)
	}
	return accounts, nil
}

func newMutableAccountState() *mutableAccountState {
	return &mutableAccountState{
		accounts: make(map[string]models.Account),
		runs:     make(map[string]int),
	}
}

func (s *mutableAccountState) runCount(action string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs[action]
}

func (s *mutableAccountState) runner() CommandRunner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		s.mu.Lock()
		defer s.mu.Unlock()

		command := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(command, " add account "):
			s.runs["add"]++
			account := models.Account{Name: valueAfterPrefix(args, "Name="), Description: valueAfterPrefix(args, "Description="), Organization: valueAfterPrefix(args, "Organization=")}
			s.accounts[account.Name] = account
			return []byte(`{"accounts":[]}`), nil
		case strings.Contains(command, " modify account "):
			s.runs["modify"]++
			account := s.accounts[valueAfterWhereName(args)]
			if desc := valueAfterPrefix(args, "Description="); desc != "" {
				account.Description = desc
			}
			if org := valueAfterPrefix(args, "Organization="); org != "" {
				account.Organization = org
			}
			s.accounts[account.Name] = account
			return []byte(`{"accounts":[]}`), nil
		case strings.Contains(command, " delete account "):
			s.runs["delete"]++
			delete(s.accounts, valueAfterWhereName(args))
			return []byte(`{"removed_accounts":[]}`), nil
		case strings.Contains(command, " list account "):
			return s.listAccountsJSON(args)
		default:
			return nil, fmt.Errorf("unexpected command: %s", command)
		}
	}
}

func (s *mutableAccountState) listAccountsJSON(args []string) ([]byte, error) {
	name := valueAfterPrefix(args, "Name=")
	if name != "" {
		if account, ok := s.accounts[name]; ok {
			return json.Marshal(map[string]any{"accounts": []models.Account{account}})
		}
		return []byte(`{"accounts":[]}`), nil
	}

	accounts := make([]models.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		accounts = append(accounts, account)
	}
	return json.Marshal(map[string]any{"accounts": accounts})
}

func valueAfterPrefix(args []string, prefix string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}

func valueAfterWhereName(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "Name=") {
			return strings.TrimPrefix(arg, "Name=")
		}
	}
	return ""
}

func stringPtr(v string) *string { return &v }

func cloneAccountsForTest(accounts []*models.Account) []*models.Account {
	cloned := make([]*models.Account, 0, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		copy := *account
		cloned = append(cloned, &copy)
	}
	return cloned
}
