package authorization

import (
	"errors"
	"fmt"

	"github.com/lingweicai/cockpit-slurm/internal/identity"
)

type OperationID string

const (
	OperationNodeDrain    OperationID = "node.drain"
	OperationNodeResume   OperationID = "node.resume"
	NodeStateHandlerID                = "node.set_state"
	NodeTargetValidatorID             = "node.target"
)

type ParameterType string

const (
	ParameterString  ParameterType = "string"
	ParameterEnum    ParameterType = "enum"
	ParameterInteger ParameterType = "integer"
	ParameterBoolean ParameterType = "boolean"
)

type ParameterSpec struct {
	Type          ParameterType
	Required      bool
	MinLength     int
	MaxLength     int
	AllowedValues []string
}

type RegistryEntry struct {
	OperationID        OperationID
	Resource           string
	Command            string
	RequiredCapability string
	TargetSchema       map[string]ParameterSpec
	ParameterSchema    map[string]ParameterSpec
	HandlerID          string
	ValidatorID        string
}

type GlobalRegistry struct {
	entries map[OperationID]RegistryEntry
}

var ErrInvalidRegistry = errors.New("invalid command operation registry")

func NewNodeRegistry() (*GlobalRegistry, error) {
	return NewGlobalRegistry(map[OperationID]RegistryEntry{
		OperationNodeDrain: {
			OperationID:        OperationNodeDrain,
			Resource:           "node",
			Command:            "update",
			RequiredCapability: identity.CapabilityNodeDrain,
			TargetSchema:       nodeTargetSchema(),
			ParameterSchema: map[string]ParameterSpec{
				"state":  {Type: ParameterEnum, Required: true, AllowedValues: []string{"DRAIN"}},
				"reason": {Type: ParameterString, Required: true, MinLength: 1, MaxLength: 256},
			},
			HandlerID:   NodeStateHandlerID,
			ValidatorID: NodeTargetValidatorID,
		},
		OperationNodeResume: {
			OperationID:        OperationNodeResume,
			Resource:           "node",
			Command:            "update",
			RequiredCapability: identity.CapabilityNodeResume,
			TargetSchema:       nodeTargetSchema(),
			ParameterSchema: map[string]ParameterSpec{
				"state": {Type: ParameterEnum, Required: true, AllowedValues: []string{"RESUME"}},
			},
			HandlerID:   NodeStateHandlerID,
			ValidatorID: NodeTargetValidatorID,
		},
	}, map[string]struct{}{
		NodeStateHandlerID: {},
	}, map[string]struct{}{
		NodeTargetValidatorID: {},
	})
}

func NewGlobalRegistry(entries map[OperationID]RegistryEntry, handlerIDs, validatorIDs map[string]struct{}) (*GlobalRegistry, error) {
	if len(entries) != 2 {
		return nil, fmt.Errorf("%w: expected exactly node.drain and node.resume", ErrInvalidRegistry)
	}

	registry := &GlobalRegistry{entries: make(map[OperationID]RegistryEntry, len(entries))}
	for key, entry := range entries {
		if key != entry.OperationID {
			return nil, fmt.Errorf("%w: key %q does not match operation %q", ErrInvalidRegistry, key, entry.OperationID)
		}
		if err := validateEntry(key, entry, handlerIDs, validatorIDs); err != nil {
			return nil, err
		}
		registry.entries[key] = cloneEntry(entry)
	}
	if drain, ok := registry.entries[OperationNodeDrain]; !ok || drain.RequiredCapability != identity.CapabilityNodeDrain {
		return nil, fmt.Errorf("%w: node.drain must require %s", ErrInvalidRegistry, identity.CapabilityNodeDrain)
	}
	if resume, ok := registry.entries[OperationNodeResume]; !ok || resume.RequiredCapability != identity.CapabilityNodeResume {
		return nil, fmt.Errorf("%w: node.resume must require %s", ErrInvalidRegistry, identity.CapabilityNodeResume)
	}
	return registry, nil
}

func (r *GlobalRegistry) Lookup(operationID OperationID) (RegistryEntry, bool) {
	if r == nil {
		return RegistryEntry{}, false
	}
	entry, ok := r.entries[operationID]
	if !ok {
		return RegistryEntry{}, false
	}
	return cloneEntry(entry), true
}

func validateEntry(key OperationID, entry RegistryEntry, handlerIDs, validatorIDs map[string]struct{}) error {
	if key != OperationNodeDrain && key != OperationNodeResume {
		return fmt.Errorf("%w: unsupported operation %q", ErrInvalidRegistry, key)
	}
	if entry.Resource != "node" || entry.Command != "update" {
		return fmt.Errorf("%w: operation %q must map to node update", ErrInvalidRegistry, key)
	}
	if entry.RequiredCapability == "" {
		return fmt.Errorf("%w: operation %q has no required capability", ErrInvalidRegistry, key)
	}
	if entry.HandlerID == "" {
		return fmt.Errorf("%w: operation %q has no handler", ErrInvalidRegistry, key)
	}
	if _, ok := handlerIDs[entry.HandlerID]; !ok {
		return fmt.Errorf("%w: operation %q references unknown handler %q", ErrInvalidRegistry, key, entry.HandlerID)
	}
	if entry.ValidatorID != "" {
		if _, ok := validatorIDs[entry.ValidatorID]; !ok {
			return fmt.Errorf("%w: operation %q references unknown validator %q", ErrInvalidRegistry, key, entry.ValidatorID)
		}
	}
	if err := validateSchema(entry.TargetSchema); err != nil {
		return fmt.Errorf("%w: target schema for %q: %v", ErrInvalidRegistry, key, err)
	}
	if err := validateSchema(entry.ParameterSchema); err != nil {
		return fmt.Errorf("%w: parameter schema for %q: %v", ErrInvalidRegistry, key, err)
	}
	return nil
}

func validateSchema(schema map[string]ParameterSpec) error {
	for name, spec := range schema {
		if name == "" {
			return errors.New("field name is empty")
		}
		switch spec.Type {
		case ParameterString, ParameterEnum, ParameterInteger, ParameterBoolean:
		default:
			return fmt.Errorf("field %q has unsupported type %q", name, spec.Type)
		}
		if spec.MinLength < 0 || spec.MaxLength < 0 || spec.MaxLength > 0 && spec.MinLength > spec.MaxLength {
			return fmt.Errorf("field %q has inconsistent length limits", name)
		}
		if spec.Type == ParameterEnum && len(spec.AllowedValues) == 0 {
			return fmt.Errorf("enum field %q has no allowed values", name)
		}
		if spec.Type != ParameterEnum && len(spec.AllowedValues) != 0 {
			return fmt.Errorf("non-enum field %q has allowed values", name)
		}
	}
	return nil
}

func nodeTargetSchema() map[string]ParameterSpec {
	return map[string]ParameterSpec{
		"nodeName": {Type: ParameterString, Required: true, MinLength: 1, MaxLength: 255},
	}
}

func cloneEntry(entry RegistryEntry) RegistryEntry {
	entry.TargetSchema = cloneSchema(entry.TargetSchema)
	entry.ParameterSchema = cloneSchema(entry.ParameterSchema)
	return entry
}

func cloneSchema(schema map[string]ParameterSpec) map[string]ParameterSpec {
	clone := make(map[string]ParameterSpec, len(schema))
	for name, spec := range schema {
		spec.AllowedValues = append([]string(nil), spec.AllowedValues...)
		clone[name] = spec
	}
	return clone
}
