package toggle

import (
	"context"
)

var defaults Toggles = defaultToggles{}

type Toggles interface {
	ProtectManagedResources() bool
	ForceFailurePolicyIgnore() bool
}

type defaultToggles struct{}

func (defaultToggles) ProtectManagedResources() bool {
	return ProtectManagedResources.enabled()
}

func (defaultToggles) ForceFailurePolicyIgnore() bool {
	return ForceFailurePolicyIgnore.enabled()
}

type contextKey struct{}

func NewContext(ctx context.Context, toggles Toggles) context.Context {
	return context.WithValue(ctx, contextKey{}, toggles)
}

func FromContext(ctx context.Context) Toggles {
	if toggles, ok := ctx.Value(contextKey{}).(Toggles); ok {
		return toggles
	}
	return defaults
}
