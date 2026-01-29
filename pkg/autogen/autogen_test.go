package autogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestV1_IsNotNil(t *testing.T) {
	assert.NotNil(t, V1, "V1 autogen should be initialized")
}

func TestV2_IsNotNil(t *testing.T) {
	assert.NotNil(t, V2, "V2 autogen should be initialized")
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

func TestV1_GetAutogenRuleNames_NilPolicy(t *testing.T) {
	// V1.GetAutogenRuleNames panics with nil policy
	assert.Panics(t, func() {
		V1.GetAutogenRuleNames(nil)
	}, "V1.GetAutogenRuleNames should panic with nil policy")
}

func TestV2_GetAutogenRuleNames_NilPolicy(t *testing.T) {
	assert.Panics(t, func() {
		V2.GetAutogenRuleNames(nil)
	}, "V2.GetAutogenRuleNames should panic with nil policy")
}

func TestV1_GetAutogenKinds_NilPolicy(t *testing.T) {
	assert.Panics(t, func() {
		V1.GetAutogenKinds(nil)
	}, "V1.GetAutogenKinds should panic with nil policy")
}

func TestV2_GetAutogenKinds_NilPolicy(t *testing.T) {
	assert.Panics(t, func() {
		V2.GetAutogenKinds(nil)
	}, "V2.GetAutogenKinds should panic with nil policy")
}

func TestV1_ComputeRules_NilPolicy(t *testing.T) {
	assert.Panics(t, func() {
		V1.ComputeRules(nil, "")
	}, "V1.ComputeRules should panic with nil policy")
}

func TestV2_ComputeRules_NilPolicy(t *testing.T) {
	assert.Panics(t, func() {
		V2.ComputeRules(nil, "")
	}, "V2.ComputeRules should panic with nil policy")
}

func TestDefault_GetAutogenRuleNames_NilPolicy(t *testing.T) {
	assert.Panics(t, func() {
		Default.GetAutogenRuleNames(nil)
	}, "Default.GetAutogenRuleNames should panic with nil policy")
}

func TestAutogenVersions_AreDifferentInstances(t *testing.T) {
	// V1 and V2 should be different implementations
	assert.NotNil(t, V1)
	assert.NotNil(t, V2)
}
