package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
)

type stubSlurmUsers struct {
	user  SlurmUser
	found bool
	err   error
	calls int
}

func (s *stubSlurmUsers) LookupUser(_ context.Context, _ string) (SlurmUser, bool, error) {
	s.calls++
	return s.user, s.found, s.err
}

func TestCapabilityResolverMapsOnlyAdminToNodeCapabilities(t *testing.T) {
	for _, test := range []struct {
		name      string
		admin     string
		coord     []string
		wantAdmin string
		wantAllow bool
	}{
		{name: "none", admin: "None", wantAdmin: "none"},
		{name: "operator", admin: "Operator", wantAdmin: "operator"},
		{name: "admin", admin: "Administrator", wantAdmin: "admin", wantAllow: true},
		{name: "coordinator only", admin: "None", coord: []string{"research"}, wantAdmin: "none"},
	} {
		t.Run(test.name, func(t *testing.T) {
			users := &stubSlurmUsers{
				user: SlurmUser{
					Username:            "slurm_test",
					AdminLevel:          test.admin,
					Accounts:            []string{"research"},
					CoordinatorAccounts: test.coord,
				},
				found: true,
			}
			resolver := NewCapabilityResolver(users)
			requester := connection.UserContext{
				UID:          1001,
				GID:          100,
				Username:     "slurm_test",
				AdminLevel:   "admin",
				Capabilities: map[string]bool{CapabilityNodeDrain: true, CapabilityNodeResume: true},
			}
			resolved, err := resolver.Resolve(context.Background(), requester)
			if err != nil {
				t.Fatalf("Resolve() returned error: %v", err)
			}
			if resolved.AdminLevel != test.wantAdmin {
				t.Fatalf("AdminLevel = %q, want %q", resolved.AdminLevel, test.wantAdmin)
			}
			if resolved.Capabilities[CapabilityNodeDrain] != test.wantAllow || resolved.Capabilities[CapabilityNodeResume] != test.wantAllow {
				t.Fatalf("capabilities = %#v, want both %t", resolved.Capabilities, test.wantAllow)
			}
			if users.calls != 1 {
				t.Fatalf("LookupUser calls = %d, want 1", users.calls)
			}
		})
	}
}

func TestCapabilityResolverFailsClosed(t *testing.T) {
	requester := connection.UserContext{UID: 1001, GID: 100, Username: "slurm_test"}
	for _, test := range []struct {
		name  string
		user  SlurmUser
		found bool
		err   error
		want  error
	}{
		{name: "missing user", found: false, want: ErrSlurmUserNotFound},
		{name: "lookup error", found: true, err: errors.New("offline"), want: ErrSlurmUserLookup},
		{name: "unknown admin level", user: SlurmUser{Username: "slurm_test", AdminLevel: "root"}, found: true, want: ErrUnknownAdminLevel},
		{name: "mismatched username", user: SlurmUser{Username: "another_user", AdminLevel: "admin"}, found: true, want: ErrIdentityUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := NewCapabilityResolver(&stubSlurmUsers{user: test.user, found: test.found, err: test.err})
			resolved, err := resolver.Resolve(context.Background(), requester)
			if !errors.Is(err, test.want) {
				t.Fatalf("Resolve() error = %v, want %v", err, test.want)
			}
			if resolved.Capabilities != nil {
				t.Fatalf("failed resolution returned capabilities: %#v", resolved.Capabilities)
			}
		})
	}

	if _, err := NewCapabilityResolver(nil).Resolve(context.Background(), requester); !errors.Is(err, ErrSlurmUserLookup) {
		t.Fatalf("nil lookup error = %v, want %v", err, ErrSlurmUserLookup)
	}
	requester.Username = ""
	if _, err := NewCapabilityResolver(&stubSlurmUsers{}).Resolve(context.Background(), requester); !errors.Is(err, ErrIdentityUnavailable) {
		t.Fatalf("missing requester username error = %v, want %v", err, ErrIdentityUnavailable)
	}
}
