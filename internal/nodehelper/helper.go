package nodehelper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/godbus/dbus/v5"
)

const (
	BusName             = "org.cockpit_project.CockpitSlurm.NodeMutation"
	ObjectPath          = dbus.ObjectPath("/org/cockpit_project/CockpitSlurm/NodeMutation")
	Interface           = "org.cockpit_project.CockpitSlurm.NodeMutation"
	ServiceAccount      = "cockpit-slurm"
	SlurmAccount        = "slurm"
	ScontrolPath        = "/usr/bin/scontrol"
	OperationNodeDrain  = "node.drain"
	OperationNodeResume = "node.resume"
	helperTimeout       = 30 * time.Second
)

var (
	ErrInvalidOperation = errors.New("unsupported node mutation operation")
	ErrInvalidTarget    = errors.New("invalid node target")
	ErrInvalidReason    = errors.New("invalid drain reason")
)

type CommandRunner interface {
	Run(ctx context.Context, executable string, args ...string) error
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, executable string, args ...string) error {
	return exec.CommandContext(ctx, executable, args...).Run()
}

type PeerUIDResolver interface {
	UID(ctx context.Context, sender string) (uint32, error)
}

type DBusService struct {
	serviceUID uint32
	peers      PeerUIDResolver
	runner     CommandRunner
}

func NewDBusService(serviceUID uint32, peers PeerUIDResolver, runner CommandRunner) *DBusService {
	if runner == nil {
		runner = execCommandRunner{}
	}
	return &DBusService{serviceUID: serviceUID, peers: peers, runner: runner}
}

func BuildScontrolArgs(operationID, nodeName, reason string) ([]string, error) {
	if !validNodeName(nodeName) {
		return nil, ErrInvalidTarget
	}
	switch operationID {
	case OperationNodeDrain:
		if err := validateDrainReason(reason); err != nil {
			return nil, err
		}
		return []string{"update", "NodeName=" + nodeName, "State=DRAIN", "Reason=" + reason}, nil
	case OperationNodeResume:
		if reason != "" {
			return nil, fmt.Errorf("%w: resume does not accept a reason", ErrInvalidReason)
		}
		return []string{"update", "NodeName=" + nodeName, "State=RESUME"}, nil
	default:
		return nil, ErrInvalidOperation
	}
}

func (s *DBusService) SetNodeState(message dbus.Message, operationID, nodeName, reason string) (string, *dbus.Error) {
	if s == nil || s.peers == nil || s.runner == nil {
		return "", dbusError("InternalError", "Node mutation helper is unavailable")
	}
	senderVariant, ok := message.Headers[dbus.FieldSender]
	if !ok {
		return "", dbusError("AccessDenied", "D-Bus caller identity is unavailable")
	}
	sender, ok := senderVariant.Value().(string)
	if !ok || sender == "" {
		return "", dbusError("AccessDenied", "D-Bus caller identity is unavailable")
	}
	callerUID, err := s.peers.UID(context.Background(), sender)
	if err != nil || callerUID != s.serviceUID {
		return "", dbusError("AccessDenied", "Only the cockpit-slurm service may request Node mutations")
	}
	args, err := BuildScontrolArgs(operationID, nodeName, reason)
	if err != nil {
		return "", dbusError("InvalidArguments", "Node mutation request is invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), helperTimeout)
	defer cancel()
	if err := s.runner.Run(ctx, ScontrolPath, args...); err != nil {
		return "", dbusError("ExecutionFailed", "Slurm rejected or failed the Node mutation")
	}
	return "succeeded", nil
}

func validNodeName(nodeName string) bool {
	if nodeName == "" || len(nodeName) > 255 || strings.EqualFold(nodeName, "ALL") {
		return false
	}
	for index, char := range nodeName {
		if index == 0 && !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return false
		}
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("_.-", char)) {
			return false
		}
	}
	return true
}

func validateDrainReason(reason string) error {
	if strings.TrimSpace(reason) == "" || len(reason) > 256 {
		return ErrInvalidReason
	}
	for _, char := range reason {
		if unicode.IsControl(char) {
			return ErrInvalidReason
		}
	}
	return nil
}

func dbusError(kind, message string) *dbus.Error {
	return dbus.NewError(Interface+".Error."+kind, []any{message})
}

type busPeerUIDResolver struct {
	conn *dbus.Conn
}

func (r busPeerUIDResolver) UID(ctx context.Context, sender string) (uint32, error) {
	var uid uint32
	call := r.conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionUnixUser", 0, sender)
	if err := call.Store(&uid); err != nil {
		return 0, fmt.Errorf("resolve D-Bus peer UID: %w", err)
	}
	return uid, nil
}

func Run(ctx context.Context) error {
	serviceUser, err := user.Lookup(ServiceAccount)
	if err != nil {
		return fmt.Errorf("resolve requesting service account: %w", err)
	}
	serviceUID, err := strconv.ParseUint(serviceUser.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("parse requesting service account UID: %w", err)
	}
	slurmUser, err := user.Lookup(SlurmAccount)
	if err != nil {
		return fmt.Errorf("resolve Slurm execution account: %w", err)
	}
	slurmUID, err := strconv.ParseUint(slurmUser.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("parse Slurm execution account UID: %w", err)
	}
	if uint64(os.Geteuid()) != slurmUID {
		return fmt.Errorf("Node mutation helper must run as SlurmUser %q (UID %d)", SlurmAccount, slurmUID)
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return fmt.Errorf("connect to system D-Bus: %w", err)
	}
	defer conn.Close()
	helperService := NewDBusService(uint32(serviceUID), busPeerUIDResolver{conn: conn}, nil)
	if err := conn.Export(helperService, ObjectPath, Interface); err != nil {
		return fmt.Errorf("export Node mutation helper: %w", err)
	}
	requestName, err := conn.RequestName(BusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("acquire D-Bus service name: %w", err)
	}
	if requestName != dbus.RequestNameReplyPrimaryOwner {
		return errors.New("Node mutation D-Bus service name is already owned")
	}
	<-ctx.Done()
	return nil
}
