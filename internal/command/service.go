package command

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"sync"
	"time"

	"github.com/lingweicai/cockpit-slurm/internal/authorization"
	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/ledger"
	"github.com/lingweicai/cockpit-slurm/internal/nodehelper"
	"github.com/lingweicai/cockpit-slurm/internal/protocol"
	"github.com/lingweicai/cockpit-slurm/internal/resource"
)

type Authorizer interface {
	Authorize(context.Context, connection.UserContext, authorization.OperationID) (authorization.Decision, error)
}

type ExecutionIdentity struct {
	UID      uint32
	Username string
	Boundary string
}

type NodeExecutor interface {
	Identity() ExecutionIdentity
	Execute(context.Context, authorization.OperationID, string, string) error
}

type Ledger interface {
	AdmissionBlocked() bool
	Reconcile(context.Context) error
	Lookup(context.Context, uint32, string) (ledger.Record, bool, error)
	ReserveAuthorization(context.Context, ledger.Claim) (ledger.Record, bool, error)
	Transition(context.Context, uint32, string, ledger.Status, ledger.Status, ledger.TransitionData) error
}

type Service struct {
	ledger      Ledger
	authorizer  Authorizer
	executor    NodeExecutor
	registry    *authorization.GlobalRegistry
	nodes       *resource.NodeCache
	refresh     func(context.Context) error
	serviceUID  uint32
	serviceUser string
	locksMu     sync.Mutex
	locks       map[string]*requestLock
}

func (s *Service) SetNodeRefresh(refresh func(context.Context) error) {
	if s != nil {
		s.refresh = refresh
	}
}

type requestLock struct {
	mu   sync.Mutex
	refs int
}

func NewService(store Ledger, authorizer Authorizer, executor NodeExecutor, registry *authorization.GlobalRegistry, nodes *resource.NodeCache) (*Service, error) {
	if store == nil || authorizer == nil || executor == nil || registry == nil || nodes == nil {
		return nil, errors.New("command service dependencies are required")
	}
	serviceUID := uint32(os.Geteuid())
	serviceUser, err := user.LookupId(strconv.Itoa(int(serviceUID)))
	if err != nil || serviceUser == nil || serviceUser.Username == "" {
		return nil, errors.New("daemon service identity is unavailable")
	}
	return &Service{
		ledger:      store,
		authorizer:  authorizer,
		executor:    executor,
		registry:    registry,
		nodes:       nodes,
		serviceUID:  serviceUID,
		serviceUser: serviceUser.Username,
		locks:       make(map[string]*requestLock),
	}, nil
}

func (s *Service) Handle(ctx context.Context, requester connection.UserContext, message protocol.Envelope) protocol.Envelope {
	if ctx == nil {
		ctx = context.Background()
	}
	request, err := protocol.DecodeNodeCommandRequestPayload(message.Payload)
	if err != nil {
		return s.errorEnvelope(message, protocol.ErrorCodeInvalidPayload, protocol.ErrorCategoryValidation, "Invalid Node command payload", false, "")
	}
	if requester.Username == "" {
		return s.errorEnvelope(message, protocol.ErrorCodeIdentityUnavailable, protocol.ErrorCategoryAuthentication, "Unable to determine the authenticated user", false, request.CommandID)
	}
	if s.ledger.AdmissionBlocked() {
		return s.persistenceError(message, request.CommandID)
	}

	canonicalPayload, err := json.Marshal(request)
	if err != nil {
		return s.errorEnvelope(message, protocol.ErrorCodeInternal, protocol.ErrorCategoryInternal, "Internal error", true, request.CommandID)
	}
	fingerprint := sha256.Sum256(canonicalPayload)
	unlock := s.lock(requester.UID, request.CommandID)
	defer unlock()

	record, found, err := s.ledger.Lookup(ctx, requester.UID, request.CommandID)
	if err != nil {
		return s.persistenceError(message, request.CommandID)
	}
	if found {
		if !equalFingerprint(record.RequestFingerprint, fingerprint[:]) {
			return s.errorEnvelope(message, protocol.ErrorCodeCommandIDConflict, protocol.ErrorCategoryConflict, "Command ID was already used for a different request", false, request.CommandID)
		}
		return s.awaitRecord(ctx, message, record, fingerprint[:])
	}

	operationID, err := operationFor(request.Parameters.State)
	if err != nil {
		return s.errorEnvelope(message, protocol.ErrorCodeInvalidCommand, protocol.ErrorCategoryValidation, "Unsupported Node operation", false, request.CommandID)
	}
	entry, ok := s.registry.Lookup(operationID)
	if !ok || entry.Resource != request.Resource || entry.Command != request.Command {
		return s.errorEnvelope(message, protocol.ErrorCodeInvalidCommand, protocol.ErrorCategoryValidation, "Unsupported Node operation", false, request.CommandID)
	}
	if err := validateAgainstRegistry(entry, request); err != nil {
		return s.errorEnvelope(message, protocol.ErrorCodeInvalidPayload, protocol.ErrorCategoryValidation, "Invalid Node command payload", false, request.CommandID)
	}
	reason := ""
	if request.Parameters.Reason != nil {
		reason = *request.Parameters.Reason
	}
	if _, err := nodehelper.BuildScontrolArgs(string(operationID), request.Target.NodeName, reason); err != nil {
		return s.errorEnvelope(message, protocol.ErrorCodeInvalidPayload, protocol.ErrorCategoryValidation, "Invalid Node command payload", false, request.CommandID)
	}
	if _, ok := s.nodes.Get(request.Target.NodeName); !ok {
		return s.errorEnvelope(message, protocol.ErrorCodeInvalidPayload, protocol.ErrorCategoryValidation, "Node target is unavailable", false, request.CommandID)
	}
	executionIdentity := s.executor.Identity()
	if executionIdentity.Username == "" || executionIdentity.Boundary == "" {
		return s.errorEnvelope(message, protocol.ErrorCodeInternal, protocol.ErrorCategoryInternal, "Command execution boundary is unavailable", true, request.CommandID)
	}

	claim := ledger.Claim{
		CallerUID:          requester.UID,
		CommandID:          request.CommandID,
		RequestFingerprint: fingerprint[:],
		RequestPayload:     canonicalPayload,
		RequestingUsername: requester.Username,
		RequestingGID:      requester.GID,
		RequestMessageID:   message.MessageID,
		ServiceUID:         s.serviceUID,
		ServiceUsername:    s.serviceUser,
		Resource:           request.Resource,
		Command:            request.Command,
		OperationID:        string(operationID),
		RequiredCapability: entry.RequiredCapability,
		TargetNode:         request.Target.NodeName,
	}
	record, created, err := s.ledger.ReserveAuthorization(ctx, claim)
	if err != nil {
		if errors.Is(err, ledger.ErrCommandIDConflict) {
			return s.errorEnvelope(message, protocol.ErrorCodeCommandIDConflict, protocol.ErrorCategoryConflict, "Command ID was already used for a different request", false, request.CommandID)
		}
		return s.persistenceError(message, request.CommandID)
	}
	if !created {
		return s.awaitRecord(ctx, message, record, fingerprint[:])
	}

	decision, err := s.authorizer.Authorize(ctx, requester, operationID)
	if err != nil {
		failure := s.errorPayload(protocol.ErrorCodeUserLookupFailed, protocol.ErrorCategoryInternal, "Slurm user information is temporarily unavailable", true, request.CommandID)
		if transitionErr := s.ledger.Transition(ctx, requester.UID, request.CommandID, ledger.StatusAuthorizationPending, ledger.StatusAuthorizationFailed, ledger.TransitionData{ErrorPayload: marshalPayload(failure)}); transitionErr != nil {
			return s.persistenceError(message, request.CommandID)
		}
		return protocol.NewErrorEnvelope(message.MessageID, failure)
	}
	if decision.RequiredCapability != entry.RequiredCapability ||
		decision.UserContext.UID != requester.UID || decision.UserContext.GID != requester.GID ||
		decision.UserContext.Username != requester.Username {
		return s.authorizationFailure(ctx, message, requester.UID, request.CommandID)
	}
	if !decision.Allowed {
		denied := s.errorPayload(protocol.ErrorCodeAuthorizationDenied, protocol.ErrorCategoryAuthorization, "Not authorized to perform this Node operation", false, request.CommandID)
		if err := s.ledger.Transition(ctx, requester.UID, request.CommandID, ledger.StatusAuthorizationPending, ledger.StatusDenied, ledger.TransitionData{ErrorPayload: marshalPayload(denied)}); err != nil {
			return s.persistenceError(message, request.CommandID)
		}
		return protocol.NewErrorEnvelope(message.MessageID, denied)
	}
	if decision.UserContext.AdminLevel != "admin" {
		return s.authorizationFailure(ctx, message, requester.UID, request.CommandID)
	}

	authorizationMetadata := marshalPayload(struct {
		RequiredCapability string    `json:"requiredCapability"`
		AuthorizedAt       time.Time `json:"authorizedAt"`
	}{RequiredCapability: entry.RequiredCapability, AuthorizedAt: time.Now().UTC()})
	if err := s.ledger.Transition(ctx, requester.UID, request.CommandID, ledger.StatusAuthorizationPending, ledger.StatusAccepted, ledger.TransitionData{
		AdminLevel:            decision.UserContext.AdminLevel,
		AuthorizationMetadata: authorizationMetadata,
	}); err != nil {
		return s.persistenceError(message, request.CommandID)
	}

	if err := s.ledger.Transition(ctx, requester.UID, request.CommandID, ledger.StatusAccepted, ledger.StatusRunning, ledger.TransitionData{
		ExecutionUID:      &executionIdentity.UID,
		ExecutionUsername: executionIdentity.Username,
		ExecutionBoundary: executionIdentity.Boundary,
	}); err != nil {
		return s.persistenceError(message, request.CommandID)
	}

	if err := s.executor.Execute(ctx, operationID, request.Target.NodeName, reason); err != nil {
		status := ledger.StatusFailed
		code, category, detail := protocol.ErrorCodeSlurmCommandFailed, protocol.ErrorCategorySlurm, "Slurm rejected the Node operation"
		if errors.Is(err, ErrExecutionOutcomeUnknown) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			status = ledger.StatusUnknown
			code, category, detail = protocol.ErrorCodeCommandOutcomeUnknown, protocol.ErrorCategoryTimeout, "The Node operation outcome is unknown; do not retry automatically"
		}
		failure := s.errorPayload(code, category, detail, false, request.CommandID)
		if transitionErr := s.ledger.Transition(ctx, requester.UID, request.CommandID, ledger.StatusRunning, status, ledger.TransitionData{ErrorPayload: marshalPayload(failure)}); transitionErr != nil {
			return s.persistenceError(message, request.CommandID)
		}
		if status == ledger.StatusUnknown {
			s.requestNodeRefresh(ctx)
		}
		return protocol.NewErrorEnvelope(message.MessageID, failure)
	}

	response := protocol.NodeCommandResponsePayload{
		CommandID:  request.CommandID,
		Resource:   request.Resource,
		Command:    request.Command,
		Target:     &request.Target,
		Parameters: &protocol.NodeCommandParameters{State: request.Parameters.State},
		Status:     protocol.CommandStatusSucceeded,
	}
	result := marshalPayload(response)
	if err := s.ledger.Transition(ctx, requester.UID, request.CommandID, ledger.StatusRunning, ledger.StatusSucceeded, ledger.TransitionData{ResultPayload: result}); err != nil {
		return s.persistenceError(message, request.CommandID)
	}
	s.requestNodeRefresh(ctx)
	return protocol.NewEnvelope(message.MessageID, protocol.MessageCommandResponse, result)
}

func (s *Service) requestNodeRefresh(ctx context.Context) {
	if s.refresh == nil {
		return
	}
	refresh := s.refresh
	go func() {
		refreshContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := refresh(refreshContext); err != nil {
			log.Printf("post-command Node refresh failed: %v", err)
		}
	}()
}

func (s *Service) HandleAuthenticated(ctx context.Context, requester connection.UserContext, message protocol.Envelope) protocol.Envelope {
	return s.Handle(ctx, requester, message)
}

func (s *Service) lock(uid uint32, commandID string) func() {
	key := fmt.Sprintf("%d:%s", uid, commandID)
	s.locksMu.Lock()
	lock := s.locks[key]
	if lock == nil {
		lock = &requestLock{}
		s.locks[key] = lock
	}
	lock.refs++
	s.locksMu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		s.locksMu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(s.locks, key)
		}
		s.locksMu.Unlock()
	}
}

func operationFor(state string) (authorization.OperationID, error) {
	switch state {
	case "DRAIN":
		return authorization.OperationNodeDrain, nil
	case "RESUME":
		return authorization.OperationNodeResume, nil
	default:
		return "", errors.New("unsupported Node state")
	}
}

func validateAgainstRegistry(entry authorization.RegistryEntry, request protocol.NodeCommandRequestPayload) error {
	if entry.OperationID == authorization.OperationNodeDrain && request.Parameters.Reason == nil {
		return errors.New("drain reason is required")
	}
	if entry.OperationID == authorization.OperationNodeResume && request.Parameters.Reason != nil {
		return errors.New("resume reason is not supported")
	}
	return nil
}

func (s *Service) authorizationFailure(ctx context.Context, message protocol.Envelope, uid uint32, commandID string) protocol.Envelope {
	failure := s.errorPayload(protocol.ErrorCodeInternal, protocol.ErrorCategoryInternal, "Authorization information is unavailable", true, commandID)
	if err := s.ledger.Transition(ctx, uid, commandID, ledger.StatusAuthorizationPending, ledger.StatusAuthorizationFailed, ledger.TransitionData{ErrorPayload: marshalPayload(failure)}); err != nil {
		return s.persistenceError(message, commandID)
	}
	return protocol.NewErrorEnvelope(message.MessageID, failure)
}

func (s *Service) recordResponse(message protocol.Envelope, record ledger.Record) protocol.Envelope {
	switch record.Status {
	case ledger.StatusSucceeded:
		return protocol.NewEnvelope(message.MessageID, protocol.MessageCommandResponse, record.ResultPayload)
	case ledger.StatusDenied, ledger.StatusAuthorizationFailed:
		var payload protocol.ErrorPayload
		if json.Unmarshal(record.ErrorPayload, &payload) == nil {
			return protocol.NewErrorEnvelope(message.MessageID, payload)
		}
	case ledger.StatusRunning:
		response := protocol.NodeCommandResponsePayload{CommandID: record.CommandID, Resource: record.Resource, Command: record.Command, Status: protocol.CommandStatusRunning}
		return protocol.NewEnvelope(message.MessageID, protocol.MessageCommandResponse, marshalPayload(response))
	case ledger.StatusFailed, ledger.StatusUnknown:
		var payload protocol.ErrorPayload
		if json.Unmarshal(record.ErrorPayload, &payload) == nil {
			return protocol.NewErrorEnvelope(message.MessageID, payload)
		}
	case ledger.StatusAuthorizationUncertain:
		return s.errorEnvelope(message, protocol.ErrorCodeAuthorizationOutcomeUnknown, protocol.ErrorCategoryInternal, "Authorization outcome is unknown; use a new command ID after recovery", false, record.CommandID)
	case ledger.StatusAuthorizationPending, ledger.StatusAccepted:
		return s.errorEnvelope(message, protocol.ErrorCodeInternal, protocol.ErrorCategoryInternal, "Command is being processed", true, record.CommandID)
	}
	return s.errorEnvelope(message, protocol.ErrorCodeInternal, protocol.ErrorCategoryInternal, "Command result is unavailable", true, record.CommandID)
}

func (s *Service) awaitRecord(ctx context.Context, message protocol.Envelope, record ledger.Record, fingerprint []byte) protocol.Envelope {
	for record.Status == ledger.StatusAuthorizationPending || record.Status == ledger.StatusAccepted {
		if s.ledger.AdmissionBlocked() {
			return s.persistenceError(message, record.CommandID)
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return s.errorEnvelope(message, protocol.ErrorCodeInternal, protocol.ErrorCategoryInternal, "Command is still being processed", true, record.CommandID)
		case <-timer.C:
		}
		updated, found, err := s.ledger.Lookup(ctx, record.CallerUID, record.CommandID)
		if err != nil {
			return s.persistenceError(message, record.CommandID)
		}
		if !found {
			return s.persistenceError(message, record.CommandID)
		}
		if !equalFingerprint(updated.RequestFingerprint, fingerprint) {
			return s.errorEnvelope(message, protocol.ErrorCodeCommandIDConflict, protocol.ErrorCategoryConflict, "Command ID was already used for a different request", false, record.CommandID)
		}
		record = updated
	}
	return s.recordResponse(message, record)
}

func (s *Service) persistenceError(message protocol.Envelope, commandID string) protocol.Envelope {
	if s.ledger.AdmissionBlocked() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := s.ledger.Reconcile(ctx); err != nil {
			log.Printf("command ledger reconciliation failed: %v", err)
		}
		cancel()
	}
	return s.errorEnvelope(message, protocol.ErrorCodeLedgerPersistenceFailed, protocol.ErrorCategoryInternal, "Command storage is unavailable; do not retry automatically", false, commandID)
}

func (s *Service) errorEnvelope(message protocol.Envelope, code protocol.ErrorCode, category protocol.ErrorCategory, detail string, retryable bool, commandID string) protocol.Envelope {
	return protocol.NewErrorEnvelope(message.MessageID, s.errorPayload(code, category, detail, retryable, commandID))
}

func (s *Service) errorPayload(code protocol.ErrorCode, category protocol.ErrorCategory, detail string, retryable bool, commandID string) protocol.ErrorPayload {
	return protocol.ErrorPayload{Code: code, Category: category, Message: detail, Retryable: retryable, CommandID: commandID}
}

func equalFingerprint(left, right []byte) bool {
	return bytes.Equal(left, right)
}

func marshalPayload(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}
