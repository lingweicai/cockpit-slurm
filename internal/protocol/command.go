package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

type NodeCommandTarget struct {
	NodeName string `json:"nodeName"`
}

type NodeCommandParameters struct {
	State  string  `json:"state"`
	Reason *string `json:"reason,omitempty"`
}

type NodeCommandRequestPayload struct {
	CommandID  string                `json:"commandId"`
	Resource   string                `json:"resource"`
	Command    string                `json:"command"`
	Target     NodeCommandTarget     `json:"target"`
	Parameters NodeCommandParameters `json:"parameters"`
}

type CommandStatus string

const (
	CommandStatusRunning   CommandStatus = "running"
	CommandStatusSucceeded CommandStatus = "succeeded"
	CommandStatusFailed    CommandStatus = "failed"
	CommandStatusDenied    CommandStatus = "denied"
	CommandStatusUnknown   CommandStatus = "unknown"
)

type NodeCommandResponsePayload struct {
	CommandID  string                 `json:"commandId"`
	Resource   string                 `json:"resource"`
	Command    string                 `json:"command"`
	Target     *NodeCommandTarget     `json:"target,omitempty"`
	Parameters *NodeCommandParameters `json:"parameters,omitempty"`
	Status     CommandStatus          `json:"status"`
}

type CommandStatusPayload struct {
	CommandID string        `json:"commandId"`
	Status    CommandStatus `json:"status"`
}

type ErrorCategory string
type ErrorCode string

const (
	ErrorCategoryAuthentication ErrorCategory = "authentication-error"
	ErrorCategoryAuthorization  ErrorCategory = "authorization-error"
	ErrorCategoryValidation     ErrorCategory = "validation-error"
	ErrorCategoryNotFound       ErrorCategory = "not-found"
	ErrorCategoryConflict       ErrorCategory = "conflict"
	ErrorCategorySlurm          ErrorCategory = "slurm-error"
	ErrorCategoryTimeout        ErrorCategory = "timeout"
	ErrorCategoryInternal       ErrorCategory = "internal-error"
	ErrorCategoryProtocol       ErrorCategory = "protocol-error"
)

const (
	ErrorCodeInvalidCommand              ErrorCode = "invalid-command"
	ErrorCodeInvalidPayload              ErrorCode = "invalid-payload"
	ErrorCodeIdentityUnavailable         ErrorCode = "identity-unavailable"
	ErrorCodeUserLookupFailed            ErrorCode = "user-lookup-failed"
	ErrorCodeAuthorizationDenied         ErrorCode = "authorization-denied"
	ErrorCodeCapabilityUnavailable       ErrorCode = "capability-unavailable"
	ErrorCodeLedgerPersistenceFailed     ErrorCode = "ledger-persistence-failed"
	ErrorCodeAuthorizationOutcomeUnknown ErrorCode = "authorization-outcome-unknown"
	ErrorCodeCommandIDConflict           ErrorCode = "command-id-conflict"
	ErrorCodeMessageIDConflict           ErrorCode = "message-id-conflict"
	ErrorCodeCommandOutcomeUnknown       ErrorCode = "command-outcome-unknown"
	ErrorCodeCommandTimeout              ErrorCode = "command-timeout"
	ErrorCodeSlurmCommandFailed          ErrorCode = "slurm-command-failed"
	ErrorCodeUnsupportedMessageType      ErrorCode = "unsupported-message-type"
	ErrorCodeInternal                    ErrorCode = "internal-error"
)

type ErrorPayload struct {
	Code      ErrorCode          `json:"code"`
	Category  ErrorCategory      `json:"category"`
	Message   string             `json:"message"`
	Retryable bool               `json:"retryable"`
	CommandID string             `json:"commandId,omitempty"`
	Resource  string             `json:"resource,omitempty"`
	Target    *NodeCommandTarget `json:"target,omitempty"`
}

func NewErrorEnvelope(messageID string, payload ErrorPayload) Envelope {
	encoded, err := json.Marshal(payload)
	if err != nil {
		encoded = []byte(`{"code":"internal-error","category":"internal-error","message":"Internal error","retryable":true}`)
	}
	return NewEnvelope(messageID, MessageError, encoded)
}

func DecodeNodeCommandRequestPayload(payload json.RawMessage) (NodeCommandRequestPayload, error) {
	var wire struct {
		CommandID  string            `json:"commandId"`
		Resource   string            `json:"resource"`
		Command    string            `json:"command"`
		Target     NodeCommandTarget `json:"target"`
		Parameters struct {
			State  string          `json:"state"`
			Reason json.RawMessage `json:"reason"`
		} `json:"parameters"`
	}
	if err := decodeStrictPayload(payload, &wire); err != nil {
		return NodeCommandRequestPayload{}, err
	}
	request := NodeCommandRequestPayload{
		CommandID: wire.CommandID,
		Resource:  wire.Resource,
		Command:   wire.Command,
		Target:    wire.Target,
		Parameters: NodeCommandParameters{
			State: wire.Parameters.State,
		},
	}
	if request.CommandID == "" || request.Resource != "node" || request.Command != "update" {
		return NodeCommandRequestPayload{}, fmt.Errorf("%w: unsupported Node command", ErrInvalidMessage)
	}
	if request.Target.NodeName == "" || len(request.Target.NodeName) > 255 {
		return NodeCommandRequestPayload{}, fmt.Errorf("%w: invalid Node target", ErrInvalidMessage)
	}
	switch request.Parameters.State {
	case "DRAIN":
		if len(wire.Parameters.Reason) == 0 || bytes.Equal(bytes.TrimSpace(wire.Parameters.Reason), []byte("null")) {
			return NodeCommandRequestPayload{}, fmt.Errorf("%w: invalid drain reason", ErrInvalidMessage)
		}
		var reason string
		if err := json.Unmarshal(wire.Parameters.Reason, &reason); err != nil {
			return NodeCommandRequestPayload{}, fmt.Errorf("%w: invalid drain reason", ErrInvalidMessage)
		}
		if strings.TrimSpace(reason) == "" || len(reason) > 256 {
			return NodeCommandRequestPayload{}, fmt.Errorf("%w: invalid drain reason", ErrInvalidMessage)
		}
		for _, char := range reason {
			if unicode.IsControl(char) {
				return NodeCommandRequestPayload{}, fmt.Errorf("%w: invalid drain reason", ErrInvalidMessage)
			}
		}
		request.Parameters.Reason = &reason
	case "RESUME":
		if len(wire.Parameters.Reason) != 0 {
			return NodeCommandRequestPayload{}, fmt.Errorf("%w: resume does not accept a reason", ErrInvalidMessage)
		}
	default:
		return NodeCommandRequestPayload{}, fmt.Errorf("%w: unsupported Node state", ErrInvalidMessage)
	}
	return request, nil
}

func DecodeNodeCommandResponsePayload(payload json.RawMessage) (NodeCommandResponsePayload, error) {
	var response NodeCommandResponsePayload
	if err := decodeStrictPayload(payload, &response); err != nil {
		return NodeCommandResponsePayload{}, err
	}
	if response.CommandID == "" || response.Resource != "node" || response.Command != "update" || !response.Status.Valid() {
		return NodeCommandResponsePayload{}, fmt.Errorf("%w: invalid command response", ErrInvalidMessage)
	}
	return response, nil
}

func DecodeCommandStatusPayload(payload json.RawMessage) (CommandStatusPayload, error) {
	var status CommandStatusPayload
	if err := decodeStrictPayload(payload, &status); err != nil {
		return CommandStatusPayload{}, err
	}
	if status.CommandID == "" || !status.Status.Valid() {
		return CommandStatusPayload{}, fmt.Errorf("%w: invalid command status", ErrInvalidMessage)
	}
	return status, nil
}

func (s CommandStatus) Valid() bool {
	switch s {
	case CommandStatusRunning, CommandStatusSucceeded, CommandStatusFailed, CommandStatusDenied, CommandStatusUnknown:
		return true
	default:
		return false
	}
}
