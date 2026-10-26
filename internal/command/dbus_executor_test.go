package command

import (
	"context"
	"errors"
	"testing"

	"github.com/lingweicai/cockpit-slurm/internal/authorization"
	"github.com/lingweicai/cockpit-slurm/internal/nodehelper"
)

type nodeMutationCallerStub struct {
	calls     int
	operation string
	node      string
	reason    string
	err       error
}

func (c *nodeMutationCallerStub) SetNodeState(_ context.Context, operationID, nodeName, reason string) error {
	c.calls++
	c.operation = operationID
	c.node = nodeName
	c.reason = reason
	return c.err
}

func TestDBusExecutorMarksUnclassifiedErrorsAsUnknown(t *testing.T) {
	caller := &nodeMutationCallerStub{err: errors.New("connection lost after dispatch")}
	executor := &DBusExecutor{caller: caller}

	err := executor.Execute(context.Background(), authorization.OperationNodeResume, "c21", "")
	if !errors.Is(err, ErrExecutionOutcomeUnknown) {
		t.Fatalf("Execute() error = %v, want ErrExecutionOutcomeUnknown", err)
	}
}

func TestDBusExecutorForwardsOnlyRegisteredNodeOperations(t *testing.T) {
	caller := &nodeMutationCallerStub{}
	executor := &DBusExecutor{
		caller: caller,
		identity: ExecutionIdentity{
			UID:      202,
			Username: nodehelper.SlurmAccount,
			Boundary: "dbus:" + nodehelper.BusName,
		},
	}

	if err := executor.Execute(context.Background(), authorization.OperationNodeDrain, "c21", "maintenance window"); err != nil {
		t.Fatalf("Execute(drain) returned error: %v", err)
	}
	if caller.calls != 1 || caller.operation != "node.drain" || caller.node != "c21" || caller.reason != "maintenance window" {
		t.Fatalf("helper call = %#v, want fixed drain operation and separate target/reason", caller)
	}
	identity := executor.Identity()
	if identity.UID != 202 || identity.Username != nodehelper.SlurmAccount || identity.Boundary != "dbus:"+nodehelper.BusName {
		t.Fatalf("execution identity = %#v, want configured Slurm helper identity", identity)
	}

	if err := executor.Execute(context.Background(), "node.power", "c21", ""); err == nil || caller.calls != 1 {
		t.Fatalf("unsupported operation error=%v helper calls=%d, want rejection before D-Bus", err, caller.calls)
	}
}
