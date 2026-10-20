package identity

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

const sacctmgrUserFixture = `{
  "users": [{
    "name": "slurm_coord",
    "administrator_level": ["None"],
    "associations": [
      {"account":"zeta","cluster":"cluster","partition":"","user":"slurm_coord","id":9},
      {"account":"devel","cluster":"cluster","partition":"","user":"slurm_coord","id":10},
      {"account":"devel","cluster":"cluster","partition":"gpu","user":"slurm_coord","id":11}
    ],
    "coordinators": [{"name":"research","direct":true}],
    "default":{"account":"devel","wckey":""},
    "flags":["NONE"],
    "old_name":"",
    "wckeys":[]
  }],
  "errors": [],
  "warnings": []
}`

func TestParseSacctmgrUserMapsSlurmFields(t *testing.T) {
	user, found, err := parseSacctmgrUser([]byte(sacctmgrUserFixture), "slurm_coord")
	if err != nil {
		t.Fatalf("parseSacctmgrUser() returned error: %v", err)
	}
	if !found {
		t.Fatal("parseSacctmgrUser() did not find the requested user")
	}
	if user.Username != "slurm_coord" || user.AdminLevel != "None" {
		t.Fatalf("Slurm identity = %#v, want slurm_coord with AdminLevel None", user)
	}
	if !reflect.DeepEqual(user.Accounts, []string{"devel", "zeta"}) {
		t.Fatalf("Accounts = %#v, want sorted unique association accounts", user.Accounts)
	}
	if !reflect.DeepEqual(user.CoordinatorAccounts, []string{"research"}) {
		t.Fatalf("CoordinatorAccounts = %#v, want research", user.CoordinatorAccounts)
	}
}

func TestParseSacctmgrUserFailsClosedOnInvalidRecords(t *testing.T) {
	for _, test := range []struct {
		name string
		json string
	}{
		{name: "malformed JSON", json: `{"users":[`},
		{name: "missing users field", json: `{"errors":[]}`},
		{name: "users is not array", json: `{"users":{}}`},
		{name: "SlurmDB error", json: `{"users":[],"errors":[{"error":"database unavailable"}]}`},
		{name: "wrong username", json: `{"users":[{"name":"another","administrator_level":["Administrator"]}],"errors":[]}`},
		{name: "multiple users", json: `{"users":[{"name":"slurm_coord"},{"name":"slurm_coord"}],"errors":[]}`},
		{name: "multiple admin levels", json: `{"users":[{"name":"slurm_coord","administrator_level":["None","Administrator"]}],"errors":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := parseSacctmgrUser([]byte(test.json), "slurm_coord"); !errors.Is(err, ErrSlurmUserLookup) {
				t.Fatalf("parseSacctmgrUser() error = %v, want %v", err, ErrSlurmUserLookup)
			}
		})
	}

	user, found, err := parseSacctmgrUser([]byte(`{"users":[],"errors":[]}`), "missing_user")
	if err != nil || found || user.Username != "" {
		t.Fatalf("missing user result = %#v, found=%t, err=%v", user, found, err)
	}
}

func TestSacctmgrUserLookupUsesFixedReadOnlyCommand(t *testing.T) {
	var gotName string
	var gotArgs []string
	lookup := &sacctmgrUserLookup{execCommandContext: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > sacctmgrLookupTimeout || time.Until(deadline) <= 0 {
			t.Fatalf("lookup context deadline = %v, want within %v", deadline, sacctmgrLookupTimeout)
		}
		return []byte(sacctmgrUserFixture), nil
	}}

	if _, found, err := lookup.LookupUser(context.Background(), "slurm_coord"); err != nil || !found {
		t.Fatalf("LookupUser() found=%t, err=%v; want found", found, err)
	}
	if gotName != sacctmgrPath {
		t.Fatalf("executable = %q, want %q", gotName, sacctmgrPath)
	}
	wantArgs := []string{"--readonly", "--json", "show", "user", "where", "name=slurm_coord", "withcoords", "withassoc"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("arguments = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestSacctmgrUserLookupRejectsInvalidUsernameBeforeExecution(t *testing.T) {
	calls := 0
	lookup := &sacctmgrUserLookup{execCommandContext: func(context.Context, string, ...string) ([]byte, error) {
		calls++
		return nil, nil
	}}
	if _, _, err := lookup.LookupUser(context.Background(), "user;modify admin"); !errors.Is(err, ErrSlurmUserLookup) {
		t.Fatalf("LookupUser() error = %v, want %v", err, ErrSlurmUserLookup)
	}
	if calls != 0 {
		t.Fatalf("executor calls = %d, want 0 for invalid username", calls)
	}
}
