package protocol

import (
	"encoding/json"
	"time"
)

const (
	ProtocolName    = "cockpit-slurm"
	ProtocolVersion = "1.0"
)

// Envelope is the common envelope for every cockpit-slurm IPC message.
type Envelope struct {
	Protocol  string          `json:"protocol"`
	Version   string          `json:"version"`
	MessageID string          `json:"messageId"`
	Type      MessageType     `json:"type"`
	Timestamp string          `json:"timestamp,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEnvelope creates a protocol envelope.
func NewEnvelope(messageID string, messageType MessageType, payload json.RawMessage) Envelope {
	return Envelope{
		Protocol:  ProtocolName,
		Version:   ProtocolVersion,
		MessageID: messageID,
		Type:      messageType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payload,
	}
}
