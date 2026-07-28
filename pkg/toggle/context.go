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
	AllowHTTPInNamespacedPolicies() bool
}

type defaultToggles struct{}

func (defaultToggles) ProtectManagedResources() bool {
	return ProtectManagedResources.Enabled()
}

func (defaultToggles) ForceFailurePolicyIgnore() bool {
	return ForceFailurePolicyIgnore.Enabled()
}

func (defaultToggles) EnableDeferredLoading() bool {
	return EnableDeferredLoading.Enabled()
}

func (defaultToggles) GenerateValidatingAdmissionPolicy() bool {
	return GenerateValidatingAdmissionPolicy.Enabled()
}

func (defaultToggles) GenerateMutatingAdmissionPolicy() bool {
	return GenerateMutatingAdmissionPolicy.Enabled()
}

func (defaultToggles) DumpMutatePatches() bool {
	return DumpMutatePatches.Enabled()
}

func (defaultToggles) AutogenV2() bool {
	return AutogenV2.Enabled()
}

func (defaultToggles) AllowHTTPInNamespacedPolicies() bool {
	return AllowHTTPInNamespacedPolicies.Enabled()
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
