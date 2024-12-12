package toggle

import (
	"context"
)

var defaults Toggles = defaultToggles{}

type Toggles interface {
	ProtectManagedResources() bool
	ForceFailurePolicyIgnore() bool
	EnableDeferredLoading() bool
	GenerateValidatingAdmissionPolicy() bool
}

type defaultToggles struct{}

func (defaultToggles) ProtectManagedResources() bool {
	return ProtectManagedResources.enabled()
}

func (defaultToggles) ForceFailurePolicyIgnore() bool {
	return ForceFailurePolicyIgnore.enabled()
}

func (defaultToggles) EnableDeferredLoading() bool {
	return EnableDeferredLoading.enabled()
}

func (defaultToggles) GenerateValidatingAdmissionPolicy() bool {
	return GenerateValidatingAdmissionPolicy.enabled()
}

type contextKey struct{}

func NewContext(ctx context.Context, toggles Toggles) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, contextKey{}, toggles)
}

func FromContext(ctx context.Context) Toggles {
	if ctx != nil {
		if toggles, ok := ctx.Value(contextKey{}).(Toggles); ok {
			return toggles
		}
	}
	return defaults
}
