package protocol

import "fmt"

var (
	ErrInvalidProtocol = fmt.Errorf("invalid protocol")
	ErrInvalidVersion  = fmt.Errorf("unsupported protocol version")
	ErrInvalidMessage  = fmt.Errorf("invalid message")
	ErrUnknownMessage  = fmt.Errorf("unsupported message type")
)

func ValidateEnvelope(e Envelope) error {
	if e.Protocol == "" {
		return fmt.Errorf("%w: missing protocol", ErrInvalidProtocol)
	}
	if e.Protocol != ProtocolName {
		return fmt.Errorf("%w: %q", ErrInvalidProtocol, e.Protocol)
	}
	if e.Version == "" {
		return fmt.Errorf("%w: missing version", ErrInvalidVersion)
	}
	if e.Version != ProtocolVersion {
		return fmt.Errorf("%w: %q", ErrInvalidVersion, e.Version)
	}
	if e.MessageID == "" {
		return fmt.Errorf("%w: missing messageId", ErrInvalidMessage)
	}
	if e.Type == "" {
		return fmt.Errorf("%w: missing type", ErrInvalidMessage)
	}
	if !e.Type.IsKnown() {
		return fmt.Errorf("%w: %q", ErrUnknownMessage, e.Type)
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("%w: missing payload", ErrInvalidMessage)
	}
	return nil
}
