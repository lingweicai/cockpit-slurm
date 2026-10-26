package nodehelper

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestDBusPolicyScopesCallsAndHelperRunsAsSlurmUser(t *testing.T) {
	policy, err := os.ReadFile("../../packaging/org.cockpit_project.CockpitSlurm.NodeMutation.conf")
	if err != nil {
		t.Fatalf("read helper D-Bus policy: %v", err)
	}
	text := string(policy)
	if !strings.Contains(text, `<policy user="cockpit-slurm">`) {
		t.Fatal("D-Bus policy does not grant access to the daemon user")
	}
	if !strings.Contains(text, `<policy user="slurm">`) || !strings.Contains(text, `<allow own="org.cockpit_project.CockpitSlurm.NodeMutation"/>`) {
		t.Fatal("D-Bus policy does not assign service ownership to SlurmUser")
	}
	if strings.Contains(text, `<policy group="cockpit-slurm">`) {
		t.Fatal("D-Bus policy grants helper access to all socket-group members")
	}
	if !strings.Contains(text, `<deny send_destination="org.cockpit_project.CockpitSlurm.NodeMutation"/>`) {
		t.Fatal("D-Bus policy does not deny helper calls by default")
	}
	unit, err := os.ReadFile("../../packaging/cockpit-slurm-node-helper.service.in")
	if err != nil {
		t.Fatalf("read helper systemd unit: %v", err)
	}
	if !strings.Contains(string(unit), "User=slurm\nGroup=slurm\n") {
		t.Fatal("helper systemd unit does not execute as SlurmUser")
	}
}

type fakePeerUIDResolver struct {
	uid uint32
	err error
}

func (r fakePeerUIDResolver) UID(context.Context, string) (uint32, error) {
	return r.uid, r.err
}

type fakeCommandRunner struct {
	path  string
	args  []string
	err   error
	calls int
}

func (r *fakeCommandRunner) Run(_ context.Context, executable string, args ...string) error {
	r.calls++
	r.path = executable
	r.args = append([]string(nil), args...)
	return r.err
}

func TestBuildScontrolArgsAllowsOnlySupportedOperations(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		node      string
		reason    string
		want      []string
	}{
		{
			name:      "drain reason remains one argument",
			operation: OperationNodeDrain,
			node:      "c21",
			reason:    "scheduled maintenance window",
			want:      []string{"update", "NodeName=c21", "State=DRAIN", "Reason=scheduled maintenance window"},
		},
		{
			name:      "shell metacharacters remain argument data",
			operation: OperationNodeDrain,
			node:      "c21",
			reason:    "maintenance; touch /tmp/should-not-run",
			want:      []string{"update", "NodeName=c21", "State=DRAIN", "Reason=maintenance; touch /tmp/should-not-run"},
		},
		{
			name:      "resume",
			operation: OperationNodeResume,
			node:      "c21",
			want:      []string{"update", "NodeName=c21", "State=RESUME"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := BuildScontrolArgs(test.operation, test.node, test.reason)
			if err != nil {
				t.Fatalf("BuildScontrolArgs() error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("arguments = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestBuildScontrolArgsRejectsBoundaryBypasses(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		node      string
		reason    string
		want      error
	}{
		{name: "unsupported operation", operation: "node.power", node: "c21", want: ErrInvalidOperation},
		{name: "arbitrary path-shaped operation", operation: "/bin/sh", node: "c21", want: ErrInvalidOperation},
		{name: "range target", operation: OperationNodeDrain, node: "c[21-24]", reason: "maintenance", want: ErrInvalidTarget},
		{name: "list target", operation: OperationNodeDrain, node: "c21,c22", reason: "maintenance", want: ErrInvalidTarget},
		{name: "ALL target", operation: OperationNodeResume, node: "ALL", want: ErrInvalidTarget},
		{name: "shell in target", operation: OperationNodeResume, node: "c21;touch", want: ErrInvalidTarget},
		{name: "shell in reason rejected as control", operation: OperationNodeDrain, node: "c21", reason: "maintenance\nState=RESUME", want: ErrInvalidReason},
		{name: "empty drain reason", operation: OperationNodeDrain, node: "c21", reason: "  ", want: ErrInvalidReason},
		{name: "oversized drain reason", operation: OperationNodeDrain, node: "c21", reason: string(make([]byte, 257)), want: ErrInvalidReason},
		{name: "reason on resume", operation: OperationNodeResume, node: "c21", reason: "ignored", want: ErrInvalidReason},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := BuildScontrolArgs(test.operation, test.node, test.reason); !errors.Is(err, test.want) {
				t.Fatalf("BuildScontrolArgs() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestDBusServiceInvokesOnlyFixedCommandForServiceUID(t *testing.T) {
	runner := &fakeCommandRunner{}
	service := NewDBusService(201, fakePeerUIDResolver{uid: 201}, runner)
	message := dbus.Message{Headers: map[dbus.HeaderField]dbus.Variant{dbus.FieldSender: dbus.MakeVariant(":1.5")}}
	result, dbusErr := service.SetNodeState(message, OperationNodeDrain, "c21", "scheduled maintenance")
	if dbusErr != nil || result != "succeeded" {
		t.Fatalf("SetNodeState() result=%q error=%v", result, dbusErr)
	}
	if runner.calls != 1 || runner.path != ScontrolPath {
		t.Fatalf("runner calls=%d path=%q, want one call to %q", runner.calls, runner.path, ScontrolPath)
	}
	want := []string{"update", "NodeName=c21", "State=DRAIN", "Reason=scheduled maintenance"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("runner args = %#v, want %#v", runner.args, want)
	}
}

func TestDBusServiceRejectsUnauthorizedAndInvalidRequestsWithoutExecution(t *testing.T) {
	runner := &fakeCommandRunner{}
	message := dbus.Message{Headers: map[dbus.HeaderField]dbus.Variant{dbus.FieldSender: dbus.MakeVariant(":1.9")}}
	service := NewDBusService(201, fakePeerUIDResolver{uid: 1008}, runner)
	if _, err := service.SetNodeState(message, OperationNodeDrain, "c21", "maintenance"); err == nil || err.Name != Interface+".Error.AccessDenied" {
		t.Fatalf("unauthorized caller error = %v, want AccessDenied", err)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0 after denied caller", runner.calls)
	}

	service = NewDBusService(201, fakePeerUIDResolver{uid: 201}, runner)
	if _, err := service.SetNodeState(message, "node.power", "c21", ""); err == nil || err.Name != Interface+".Error.InvalidArguments" {
		t.Fatalf("invalid operation error = %v, want InvalidArguments", err)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0 for invalid operation", runner.calls)
	}
}

func TestDBusServiceRejectsMissingSenderWithoutExecution(t *testing.T) {
	runner := &fakeCommandRunner{}
	service := NewDBusService(201, fakePeerUIDResolver{uid: 201}, runner)
	if _, err := service.SetNodeState(dbus.Message{}, OperationNodeResume, "c21", ""); err == nil || err.Name != Interface+".Error.AccessDenied" {
		t.Fatalf("missing sender error = %v, want AccessDenied", err)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0", runner.calls)
	}
}

func TestDBusServiceReturnsClassifiedExecutionFailure(t *testing.T) {
	runner := &fakeCommandRunner{err: errors.New("private raw command output")}
	service := NewDBusService(201, fakePeerUIDResolver{uid: 201}, runner)
	message := dbus.Message{Headers: map[dbus.HeaderField]dbus.Variant{dbus.FieldSender: dbus.MakeVariant(":1.5")}}
	if _, err := service.SetNodeState(message, OperationNodeResume, "c21", ""); err == nil || err.Name != Interface+".Error.ExecutionFailed" {
		t.Fatalf("execution failure = %v, want classified error", err)
	}
	if runner.calls != 1 || runner.path != ScontrolPath {
		t.Fatalf("runner calls=%d path=%q, want fixed command execution", runner.calls, runner.path)
	}
}
