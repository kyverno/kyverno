package exceptions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{
			name:     "maxRetries constant",
			got:      maxRetries,
			expected: 10,
		},
		{
			name:     "Workers constant",
			got:      Workers,
			expected: 3,
		},
		{
			name:     "ControllerName constant",
			got:      ControllerName,
			expected: "exceptions-controller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.got)
		})
	}
}

func TestControllerNameValue(t *testing.T) {
	assert.NotEmpty(t, ControllerName)
	assert.Equal(t, "exceptions-controller", ControllerName)
}

func TestWorkersValue(t *testing.T) {
	assert.Equal(t, 3, Workers)
	assert.Greater(t, Workers, 0)
}

func TestMaxRetriesValue(t *testing.T) {
	assert.Equal(t, 10, maxRetries)
	assert.Greater(t, maxRetries, 0)
}

func TestRuleIndexType(t *testing.T) {
	// Test that ruleIndex can be created and used
	idx := make(ruleIndex)
	assert.NotNil(t, idx)
	assert.Empty(t, idx)
}

func TestPolicyIndexType(t *testing.T) {
	// Test that policyIndex can be created and used
	idx := make(policyIndex)
	assert.NotNil(t, idx)
	assert.Empty(t, idx)
}

func TestRuleIndexOperations(t *testing.T) {
	idx := make(ruleIndex)

	// Test adding to the index
	idx["test-rule"] = nil
	assert.Len(t, idx, 1)

	// Test key exists
	_, exists := idx["test-rule"]
	assert.True(t, exists)

	// Test key doesn't exist
	_, exists = idx["nonexistent"]
	assert.False(t, exists)

	// Test deletion
	delete(idx, "test-rule")
	assert.Empty(t, idx)
}

func TestPolicyIndexOperations(t *testing.T) {
	idx := make(policyIndex)

	// Test adding to the index
	idx["test-policy"] = make(ruleIndex)
	assert.Len(t, idx, 1)

	// Test nested operations
	idx["test-policy"]["test-rule"] = nil
	assert.Len(t, idx["test-policy"], 1)

	// Test key exists
	_, exists := idx["test-policy"]
	assert.True(t, exists)

	// Test key doesn't exist
	_, exists = idx["nonexistent"]
	assert.False(t, exists)

	// Test deletion
	delete(idx, "test-policy")
	assert.Empty(t, idx)
}

func TestPolicyIndexNested(t *testing.T) {
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
}
