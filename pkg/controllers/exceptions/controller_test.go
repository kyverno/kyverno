package exceptions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConstants verifies the controller configuration constants are set correctly
func TestConstants(t *testing.T) {
	t.Run("maxRetries", func(t *testing.T) {
		assert.Equal(t, maxRetries, 10)
		assert.Greater(t, maxRetries, 0)
	})

	t.Run("Workers", func(t *testing.T) {
		assert.Equal(t, Workers, 3)
		assert.Greater(t, Workers, 0)
	})

	t.Run("ControllerName", func(t *testing.T) {
		assert.Equal(t, ControllerName, "exceptions-controller")
		assert.NotEmpty(t, ControllerName)
	})
}

func TestIndexTypes(t *testing.T) {
	t.Run("ruleIndex creation", func(t *testing.T) {
		idx := make(ruleIndex)
		assert.NotNil(t, idx)
		assert.Empty(t, idx)
	})

	t.Run("policyIndex creation", func(t *testing.T) {
		idx := make(policyIndex)
		assert.NotNil(t, idx)
		assert.Empty(t, idx)
	})
}

func TestPolicyIndexNestedStructure(t *testing.T) {
	idx := make(policyIndex)

	// Create nested structure
	idx["policy1"] = make(ruleIndex)
	idx["policy1"]["rule1"] = nil
	idx["policy1"]["rule2"] = nil

	idx["policy2"] = make(ruleIndex)
	idx["policy2"]["rule1"] = nil

	assert.Len(t, idx, 2)
	assert.Len(t, idx["policy1"], 2)
	assert.Len(t, idx["policy2"], 1)

	// Test key exists
	_, exists := idx["policy1"]
	assert.True(t, exists)

	// Test deletion
	delete(idx, "policy1")
	assert.Len(t, idx, 1)
}
