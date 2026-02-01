package autogen

import (
	"testing"

	"gotest.tools/assert"
)

func TestV1_IsNotNil(t *testing.T) {
	assert.Assert(t, V1 != nil, "V1 autogen should be initialized")
}

func TestV2_IsNotNil(t *testing.T) {
	assert.Assert(t, V2 != nil, "V2 autogen should be initialized")
}

func TestDefault_IsV1(t *testing.T) {
	assert.Equal(t, V1, Default, "Default autogen should be V1")
}

func TestAutogenInterface_V1ImplementsInterface(t *testing.T) {
	var _ Autogen = V1
}

func TestAutogenInterface_V2ImplementsInterface(t *testing.T) {
	var _ Autogen = V2
}

func TestNilPolicy_Panics(t *testing.T) {
	// These functions are expected to panic when called with nil policy
	// because they call p.GetSpec() on a nil pointer
	tests := []struct {
		name string
		fn   func()
	}{
		{"V1.GetAutogenRuleNames", func() { V1.GetAutogenRuleNames(nil) }},
		{"V2.GetAutogenRuleNames", func() { V2.GetAutogenRuleNames(nil) }},
		{"V1.GetAutogenKinds", func() { V1.GetAutogenKinds(nil) }},
		{"V2.GetAutogenKinds", func() { V2.GetAutogenKinds(nil) }},
		{"V1.ComputeRules", func() { V1.ComputeRules(nil, "") }},
		{"V2.ComputeRules", func() { V2.ComputeRules(nil, "") }},
		{"Default.GetAutogenRuleNames", func() { Default.GetAutogenRuleNames(nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("%s should panic with nil policy", tt.name)
				}
			}()
			tt.fn()
		})
	}
}

func TestAutogenVersions_AreDifferent(t *testing.T) {
	assert.Assert(t, V1 != V2, "V1 and V2 should be different implementations")
}
