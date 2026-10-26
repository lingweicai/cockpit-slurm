package authorization

import (
	"context"
	"errors"
	"fmt"

	"github.com/lingweicai/cockpit-slurm/internal/connection"
	"github.com/lingweicai/cockpit-slurm/internal/identity"
)

var (
	ErrUnknownOperation    = errors.New("unknown command operation")
	ErrResolverUnavailable = errors.New("capability resolver unavailable")
)

type CapabilityResolver interface {
	Resolve(ctx context.Context, requester connection.UserContext) (connection.UserContext, error)
}

type Decision struct {
	Allowed            bool
	RequiredCapability string
	UserContext        connection.UserContext
}

type Authorizer struct {
	registry *GlobalRegistry
	resolver CapabilityResolver
}

func NewAuthorizer(registry *GlobalRegistry, resolver CapabilityResolver) *Authorizer {
	return &Authorizer{registry: registry, resolver: resolver}
}

func (a *Authorizer) Authorize(ctx context.Context, requester connection.UserContext, operationID OperationID) (Decision, error) {
	if a == nil || a.registry == nil {
		return Decision{}, fmt.Errorf("%w: registry unavailable", ErrResolverUnavailable)
	}
	entry, ok := a.registry.Lookup(operationID)
	if !ok {
		return Decision{}, fmt.Errorf("%w: %q", ErrUnknownOperation, operationID)
	}
	if a.resolver == nil {
		return Decision{}, ErrResolverUnavailable
	}

	current, err := a.resolver.Resolve(ctx, requester)
	if err != nil {
		return Decision{}, err
	}
	if requester.Username == "" || current.Username != requester.Username || current.UID != requester.UID || current.GID != requester.GID {
		return Decision{}, fmt.Errorf("%w: resolver returned a different identity", identity.ErrIdentityUnavailable)
	}
	return Decision{
		Allowed:            current.Capabilities[entry.RequiredCapability],
		RequiredCapability: entry.RequiredCapability,
		UserContext:        current,
	}, nil
}
