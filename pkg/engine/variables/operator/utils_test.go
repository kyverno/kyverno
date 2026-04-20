package operator

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

func TestGetAllConditionOperators(t *testing.T) {
	operators := GetAllConditionOperators()

	// Should return all condition operators from the kyvernov1 package
	assert.NotEmpty(t, operators)

	// Check that some known operators are present
	expectedOperators := []string{
		"Equals",
		"NotEquals",
		"GreaterThan",
		"GreaterThanOrEquals",
		"LessThan",
		"LessThanOrEquals",
		"AllIn",
		"AnyIn",
		"AllNotIn",
		"AnyNotIn",
	}

	for _, expected := range expectedOperators {
		assert.Contains(t, operators, expected, "Expected operator %s to be in the list", expected)
	}
}

func TestGetAllDeprecatedOperators(t *testing.T) {
	operators := GetAllDeprecatedOperators()

	// Should return deprecated operators
	assert.NotEmpty(t, operators)

	// Check known deprecated operators
	assert.Contains(t, operators, "In")
	assert.Contains(t, operators, "NotIn")
}

func TestGetDeprecatedOperatorAlternative(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		expected []string
	}{
		{
			name:     "In has alternatives",
			operator: "In",
			expected: []string{"AllIn", "AnyIn"},
		},
		{
			name:     "NotIn has alternatives",
			operator: "NotIn",
			expected: []string{"AllNotIn", "AnyNotIn"},
		},
		{
			name:     "Unknown operator has no alternatives",
			operator: "Unknown",
			expected: []string{},
		},
		{
			name:     "Empty operator has no alternatives",
			operator: "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDeprecatedOperatorAlternative(tt.operator)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsOperatorValid(t *testing.T) {
	tests := []struct {
		name     string
		operator kyvernov1.ConditionOperator
		expected bool
	}{
		{
			name:     "Equals is valid",
			operator: kyvernov1.ConditionOperators["Equals"],
			expected: true,
		},
		{
			name:     "NotEquals is valid",
			operator: kyvernov1.ConditionOperators["NotEquals"],
			expected: true,
		},
		{
			name:     "GreaterThan is valid",
			operator: kyvernov1.ConditionOperators["GreaterThan"],
			expected: true,
		},
		{
			name:     "GreaterThanOrEquals is valid",
			operator: kyvernov1.ConditionOperators["GreaterThanOrEquals"],
			expected: true,
		},
		{
			name:     "LessThan is valid",
			operator: kyvernov1.ConditionOperators["LessThan"],
			expected: true,
		},
		{
			name:     "LessThanOrEquals is valid",
			operator: kyvernov1.ConditionOperators["LessThanOrEquals"],
			expected: true,
		},
		{
			name:     "AllIn is valid",
			operator: kyvernov1.ConditionOperators["AllIn"],
			expected: true,
		},
		{
			name:     "AnyIn is valid",
			operator: kyvernov1.ConditionOperators["AnyIn"],
			expected: true,
		},
		{
			name:     "AllNotIn is valid",
			operator: kyvernov1.ConditionOperators["AllNotIn"],
			expected: true,
		},
		{
			name:     "AnyNotIn is valid",
			operator: kyvernov1.ConditionOperators["AnyNotIn"],
			expected: true,
		},
		{
			name:     "Invalid operator",
			operator: kyvernov1.ConditionOperator("InvalidOperator"),
			expected: false,
		},
		{
			name:     "Empty operator",
			operator: kyvernov1.ConditionOperator(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOperatorValid(tt.operator)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsOperatorDeprecated(t *testing.T) {
	tests := []struct {
		name     string
		operator kyvernov1.ConditionOperator
		expected bool
	}{
		{
			name:     "In is deprecated",
			operator: kyvernov1.ConditionOperator("In"),
			expected: true,
		},
		{
			name:     "NotIn is deprecated",
			operator: kyvernov1.ConditionOperator("NotIn"),
			expected: true,
		},
		{
			name:     "Equals is not deprecated",
			operator: kyvernov1.ConditionOperators["Equals"],
			expected: false,
		},
		{
			name:     "AllIn is not deprecated",
			operator: kyvernov1.ConditionOperators["AllIn"],
			expected: false,
		},
		{
			name:     "AnyIn is not deprecated",
			operator: kyvernov1.ConditionOperators["AnyIn"],
			expected: false,
		},
		{
			name:     "AllNotIn is not deprecated",
			operator: kyvernov1.ConditionOperators["AllNotIn"],
			expected: false,
		},
		{
			name:     "AnyNotIn is not deprecated",
			operator: kyvernov1.ConditionOperators["AnyNotIn"],
			expected: false,
		},
		{
			name:     "Unknown operator is not deprecated",
			operator: kyvernov1.ConditionOperator("Unknown"),
			expected: false,
		},
		{
			name:     "Empty operator is not deprecated",
			operator: kyvernov1.ConditionOperator(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOperatorDeprecated(tt.operator)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeprecatedOperatorsMap(t *testing.T) {
	// Verify the deprecatedOperators map structure
	expectedMap := map[string][]string{
		"In":    {"AllIn", "AnyIn"},
		"NotIn": {"AllNotIn", "AnyNotIn"},
	}

	for op, alternatives := range expectedMap {
		result := GetDeprecatedOperatorAlternative(op)
		assert.Equal(t, alternatives, result, "Alternatives for %s should match", op)
	}
}
