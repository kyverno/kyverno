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
	GenerateMutatingAdmissionPolicy() bool
	DumpMutatePatches() bool
	AutogenV2() bool
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

func (defaultToggles) GenerateMutatingAdmissionPolicy() bool {
	return GenerateMutatingAdmissionPolicy.enabled()
}

func (defaultToggles) DumpMutatePatches() bool {
	return DumpMutatePatches.enabled()
}

func (defaultToggles) AutogenV2() bool {
	return AutogenV2.enabled()
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
