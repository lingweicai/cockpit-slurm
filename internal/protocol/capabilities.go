package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type CapabilitiesRequestPayload struct{}

type NodeCapabilities struct {
	NodesDrain  bool `json:"nodes.drain"`
	NodesResume bool `json:"nodes.resume"`
}

type CapabilitiesResponsePayload struct {
	Capabilities NodeCapabilities `json:"capabilities"`
}

func DecodeCapabilitiesRequestPayload(payload json.RawMessage) (CapabilitiesRequestPayload, error) {
	var request CapabilitiesRequestPayload
	if err := decodeStrictPayload(payload, &request); err != nil {
		return CapabilitiesRequestPayload{}, err
	}
	return request, nil
}

func DecodeCapabilitiesResponsePayload(payload json.RawMessage) (CapabilitiesResponsePayload, error) {
	var wire struct {
		Capabilities *struct {
			NodesDrain  *bool `json:"nodes.drain"`
			NodesResume *bool `json:"nodes.resume"`
		} `json:"capabilities"`
	}
	if err := decodeStrictPayload(payload, &wire); err != nil {
		return CapabilitiesResponsePayload{}, err
	}
	if wire.Capabilities == nil || wire.Capabilities.NodesDrain == nil || wire.Capabilities.NodesResume == nil {
		return CapabilitiesResponsePayload{}, fmt.Errorf("%w: capabilities must contain nodes.drain and nodes.resume", ErrInvalidMessage)
	}

	return CapabilitiesResponsePayload{Capabilities: NodeCapabilities{
		NodesDrain:  *wire.Capabilities.NodesDrain,
		NodesResume: *wire.Capabilities.NodesResume,
	}}, nil
}

func decodeStrictPayload(payload []byte, destination any) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("%w: payload must be an object", ErrInvalidMessage)
	}
	if err := validateUniqueJSONKeys(trimmed); err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: invalid payload: %v", ErrInvalidMessage, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%w: trailing payload data", ErrInvalidMessage)
	}
	return nil
}
