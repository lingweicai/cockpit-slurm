package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSacctmgrAccountProviderListAccounts(t *testing.T) {
	t.Helper()

	var gotName string
	var gotArgs []string
	provider := NewSacctmgrAccountProviderWithRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte(`{"accounts":[{"name":"alpha","description":"Alpha account","organization":"org1"},{"name":"beta","description":"Beta account","organization":"org2"}]}`), nil
	})

	accounts, err := provider.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}

	if gotName != "sacctmgr" {
		t.Fatalf("command name = %q, want %q", gotName, "sacctmgr")
	}

	wantArgs := []string{"list", "account", "--json"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("args length = %d, want %d (%v)", len(gotArgs), len(wantArgs), gotArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Fatalf("arg[%d] = %q, want %q", i, gotArgs[i], wantArgs[i])
		}
	}

	if len(accounts) != 2 {
		t.Fatalf("accounts length = %d, want 2", len(accounts))
	}
	if accounts[0].Name != "alpha" || accounts[1].Name != "beta" {
		t.Fatalf("accounts = %#v", accounts)
	}
}

func TestSacctmgrAccountProviderListAccountsCommandError(t *testing.T) {
	provider := NewSacctmgrAccountProviderWithRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("permission denied")
	})

	_, err := provider.ListAccounts(context.Background())
	if err == nil {
		t.Fatal("ListAccounts() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "execute sacctmgr list account --json") {
		t.Fatalf("error = %q, want command context", err.Error())
	}
}

func TestSacctmgrAccountProviderListAccountsParseError(t *testing.T) {
	provider := NewSacctmgrAccountProviderWithRunner(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`{"accounts":`), nil
	})

	_, err := provider.ListAccounts(context.Background())
	if err == nil {
		t.Fatal("ListAccounts() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse sacctmgr list account JSON") {
		t.Fatalf("error = %q, want parse context", err.Error())
	}
}
