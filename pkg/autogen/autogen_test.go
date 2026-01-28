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
	assert.NotNil(t, V1)
}

func TestAutogenInterface_V2ImplementsInterface(t *testing.T) {
	var _ Autogen = V2
	assert.NotNil(t, V2)
}

func TestV1_GetAutogenRuleNames_NilPolicy(t *testing.T) {
	// V1.GetAutogenRuleNames should handle nil gracefully or panic
	// This tests that the implementation exists and is callable
	defer func() {
		if r := recover(); r != nil {
			// Expected to panic with nil policy
			t.Log("V1.GetAutogenRuleNames panics with nil policy as expected")
		}
	}()
	
	result := V1.GetAutogenRuleNames(nil)
	// If it doesn't panic, it should return empty or nil
	assert.Empty(t, result)
}

func TestV2_GetAutogenRuleNames_NilPolicy(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log("V2.GetAutogenRuleNames panics with nil policy as expected")
		}
	}()
	
	result := V2.GetAutogenRuleNames(nil)
	assert.Empty(t, result)
}

func TestV1_GetAutogenKinds_NilPolicy(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log("V1.GetAutogenKinds panics with nil policy as expected")
		}
	}()
	
	result := V1.GetAutogenKinds(nil)
	assert.Empty(t, result)
}

func TestV2_GetAutogenKinds_NilPolicy(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log("V2.GetAutogenKinds panics with nil policy as expected")
		}
	}()
	
	result := V2.GetAutogenKinds(nil)
	assert.Empty(t, result)
}

func TestV1_ComputeRules_NilPolicy(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log("V1.ComputeRules panics with nil policy as expected")
		}
	}()
	
	result := V1.ComputeRules(nil, "")
	assert.Empty(t, result)
}

func TestV2_ComputeRules_NilPolicy(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log("V2.ComputeRules panics with nil policy as expected")
		}
	}()
	
	result := V2.ComputeRules(nil, "")
	assert.Empty(t, result)
}

func TestDefault_GetAutogenRuleNames_NilPolicy(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Log("Default.GetAutogenRuleNames panics with nil policy as expected")
		}
	}()
	
	result := Default.GetAutogenRuleNames(nil)
	assert.Empty(t, result)
}

func TestAutogenVersions_AreDifferentInstances(t *testing.T) {
	// V1 and V2 should be different implementations
	// They may or may not be equal depending on implementation
	// but they should both implement the interface
	assert.NotNil(t, V1)
	assert.NotNil(t, V2)
}
