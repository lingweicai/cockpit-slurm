package command

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"strconv"

	"github.com/godbus/dbus/v5"
	"github.com/lingweicai/cockpit-slurm/internal/authorization"
	"github.com/lingweicai/cockpit-slurm/internal/nodehelper"
)

var ErrExecutionOutcomeUnknown = errors.New("Node execution outcome is unknown")

type nodeMutationCaller interface {
	SetNodeState(context.Context, string, string, string) error
}

type DBusExecutor struct {
	caller   nodeMutationCaller
	identity ExecutionIdentity
	conn     *dbus.Conn
}

func NewDBusExecutor() (*DBusExecutor, error) {
	account, err := user.Lookup(nodehelper.SlurmAccount)
	if err != nil {
		return nil, fmt.Errorf("resolve Node helper execution identity: %w", err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("parse Node helper execution UID: %w", err)
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect to system D-Bus for Node commands: %w", err)
	}
	return &DBusExecutor{
		caller: dbusNodeMutationCaller{object: conn.Object(nodehelper.BusName, nodehelper.ObjectPath)},
		identity: ExecutionIdentity{
			UID:      uint32(uid),
			Username: account.Username,
			Boundary: "dbus:" + nodehelper.BusName,
		},
		conn: conn,
	}, nil
}

func (e *DBusExecutor) Identity() ExecutionIdentity {
	if e == nil {
		return ExecutionIdentity{}
	}
	return e.identity
}

func (e *DBusExecutor) Execute(ctx context.Context, operation authorization.OperationID, nodeName, reason string) error {
	if e == nil || e.caller == nil {
		return errors.New("Node helper D-Bus client is unavailable")
	}
	if operation != authorization.OperationNodeDrain && operation != authorization.OperationNodeResume {
		return nodehelper.ErrInvalidOperation
	}
	err := e.caller.SetNodeState(ctx, string(operation), nodeName, reason)
	if err == nil || errors.Is(err, ErrExecutionOutcomeUnknown) {
		return err
	}
	var dbusErr *dbus.Error
	if errors.As(err, &dbusErr) && (dbusErr.Name == nodehelper.Interface+".Error.InvalidArguments" ||
		dbusErr.Name == nodehelper.Interface+".Error.AccessDenied" ||
		dbusErr.Name == nodehelper.Interface+".Error.ExecutionFailed") {
		return err
	}
	return fmt.Errorf("%w: %v", ErrExecutionOutcomeUnknown, err)
}

func (e *DBusExecutor) Close() error {
	if e == nil || e.conn == nil {
		return nil
	}
	return e.conn.Close()
}

type dbusNodeMutationCaller struct {
	object dbus.BusObject
}

func (c dbusNodeMutationCaller) SetNodeState(ctx context.Context, operationID, nodeName, reason string) error {
	if c.object == nil {
		return errors.New("Node helper D-Bus object is unavailable")
	}
	call := c.object.CallWithContext(ctx, nodehelper.Interface+".SetNodeState", 0, operationID, nodeName, reason)
	var result string
	if err := call.Store(&result); err != nil {
		return fmt.Errorf("call restricted Node mutation helper: %w", err)
	}
	if result != "succeeded" {
		return errors.New("restricted Node mutation helper returned an invalid result")
	}
	return nil
}
