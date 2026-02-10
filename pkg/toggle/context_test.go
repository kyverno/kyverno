package toggle

import (
	"context"
	"testing"
)

// mockToggles implements Toggles interface for testing
type mockToggles struct {
	protectManagedResources           bool
	forceFailurePolicyIgnore          bool
	enableDeferredLoading             bool
	generateValidatingAdmissionPolicy bool
	generateMutatingAdmissionPolicy   bool
	dumpMutatePatches                 bool
	autogenV2                         bool
	unifiedImageVerifiers             bool
}

func (m mockToggles) ProtectManagedResources() bool  { return m.protectManagedResources }
func (m mockToggles) ForceFailurePolicyIgnore() bool { return m.forceFailurePolicyIgnore }
func (m mockToggles) EnableDeferredLoading() bool    { return m.enableDeferredLoading }
func (m mockToggles) GenerateValidatingAdmissionPolicy() bool {
	return m.generateValidatingAdmissionPolicy
}
func (m mockToggles) GenerateMutatingAdmissionPolicy() bool { return m.generateMutatingAdmissionPolicy }
func (m mockToggles) DumpMutatePatches() bool               { return m.dumpMutatePatches }
func (m mockToggles) AutogenV2() bool                       { return m.autogenV2 }
func (m mockToggles) UnifiedImageVerifiers() bool           { return m.unifiedImageVerifiers }

func TestNewContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		toggles Toggles
		wantNil bool
	}{
		{
			name:    "nil context returns nil",
			ctx:     nil,
			toggles: mockToggles{},
			wantNil: true,
		},
		{
			name:    "valid context with toggles",
			ctx:     context.Background(),
			toggles: mockToggles{protectManagedResources: true},
			wantNil: false,
		},
		{
			name: "context with all toggles enabled",
			ctx:  context.Background(),
			toggles: mockToggles{
				protectManagedResources:           true,
				forceFailurePolicyIgnore:          true,
				enableDeferredLoading:             true,
				generateValidatingAdmissionPolicy: true,
				generateMutatingAdmissionPolicy:   true,
				dumpMutatePatches:                 true,
				autogenV2:                         true,
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewContext(tt.ctx, tt.toggles)
			if tt.wantNil && result != nil {
				t.Errorf("NewContext() = %v, want nil", result)
			}
			if !tt.wantNil && result == nil {
				t.Errorf("NewContext() = nil, want non-nil context")
			}
		})
	}
}

func TestFromContext(t *testing.T) {
	tests := []struct {
		name                   string
		setupCtx               func() context.Context
		wantProtectManaged     bool
		wantForceFailureIgnore bool
	}{
		{
			name: "nil context returns defaults",
			setupCtx: func() context.Context {
				return nil
			},
			wantProtectManaged:     false,
			wantForceFailureIgnore: false,
		},
		{
			name: "context without toggles returns defaults",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantProtectManaged:     false,
			wantForceFailureIgnore: false,
		},
		{
			name: "context with custom toggles",
			setupCtx: func() context.Context {
				toggles := mockToggles{
					protectManagedResources:  true,
					forceFailurePolicyIgnore: true,
				}
				return NewContext(context.Background(), toggles)
			},
			wantProtectManaged:     true,
			wantForceFailureIgnore: true,
		},
		{
			name: "context with partial toggles",
			setupCtx: func() context.Context {
				toggles := mockToggles{
					protectManagedResources:  true,
					forceFailurePolicyIgnore: false,
				}
				return NewContext(context.Background(), toggles)
			},
			wantProtectManaged:     true,
			wantForceFailureIgnore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := FromContext(ctx)

			if result == nil {
				t.Fatal("FromContext() returned nil")
			}

			if got := result.ProtectManagedResources(); got != tt.wantProtectManaged {
				t.Errorf("ProtectManagedResources() = %v, want %v", got, tt.wantProtectManaged)
			}

			if got := result.ForceFailurePolicyIgnore(); got != tt.wantForceFailureIgnore {
				t.Errorf("ForceFailurePolicyIgnore() = %v, want %v", got, tt.wantForceFailureIgnore)
			}
		})
	}
}

func TestFromContextReturnsStoredToggles(t *testing.T) {
	expected := mockToggles{
		protectManagedResources:           true,
		forceFailurePolicyIgnore:          true,
		enableDeferredLoading:             true,
		generateValidatingAdmissionPolicy: true,
		generateMutatingAdmissionPolicy:   true,
		dumpMutatePatches:                 true,
		autogenV2:                         true,
	}

	ctx := NewContext(context.Background(), expected)
	result := FromContext(ctx)

	if result.ProtectManagedResources() != expected.ProtectManagedResources() {
		t.Error("ProtectManagedResources mismatch")
	}
	if result.ForceFailurePolicyIgnore() != expected.ForceFailurePolicyIgnore() {
		t.Error("ForceFailurePolicyIgnore mismatch")
	}
	if result.EnableDeferredLoading() != expected.EnableDeferredLoading() {
		t.Error("EnableDeferredLoading mismatch")
	}
	if result.GenerateValidatingAdmissionPolicy() != expected.GenerateValidatingAdmissionPolicy() {
		t.Error("GenerateValidatingAdmissionPolicy mismatch")
	}
	if result.GenerateMutatingAdmissionPolicy() != expected.GenerateMutatingAdmissionPolicy() {
		t.Error("GenerateMutatingAdmissionPolicy mismatch")
	}
	if result.DumpMutatePatches() != expected.DumpMutatePatches() {
		t.Error("DumpMutatePatches mismatch")
	}
	if result.AutogenV2() != expected.AutogenV2() {
		t.Error("AutogenV2 mismatch")
	}
}

func TestDefaultToggles(t *testing.T) {
	// Test that FromContext with empty context returns default toggles
	result := FromContext(context.Background())

	if result == nil {
		t.Fatal("FromContext with background context should not return nil")
	}

	// Default toggles should be returned - verify the interface is implemented
	_ = result.ProtectManagedResources()
	_ = result.ForceFailurePolicyIgnore()
	_ = result.EnableDeferredLoading()
	_ = result.GenerateValidatingAdmissionPolicy()
	_ = result.GenerateMutatingAdmissionPolicy()
	_ = result.DumpMutatePatches()
	_ = result.AutogenV2()
}

func TestContextChaining(t *testing.T) {
	// Test that we can chain contexts properly
	toggles1 := mockToggles{protectManagedResources: true}
	ctx1 := NewContext(context.Background(), toggles1)

	toggles2 := mockToggles{forceFailurePolicyIgnore: true}
	ctx2 := NewContext(ctx1, toggles2)

	// The most recent context should win
	result := FromContext(ctx2)
	if result.ProtectManagedResources() != false {
		t.Error("Expected ProtectManagedResources to be false from newer context")
	}
	if result.ForceFailurePolicyIgnore() != true {
		t.Error("Expected ForceFailurePolicyIgnore to be true from newer context")
	}
}
