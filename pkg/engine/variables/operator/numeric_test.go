package operator

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

func TestNumericOperatorHandler_Evaluate_GreaterThan(t *testing.T) {
	log := logr.Discard()
	handler := NewNumericOperatorHandler(log, nil, kyvernov1.ConditionOperators["GreaterThan"])

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		// Int key comparisons
		{
			name:     "int > int true",
			key:      10,
			value:    5,
			expected: true,
		},
		{
			name:     "int > int false",
			key:      5,
			value:    10,
			expected: false,
		},
		{
			name:     "int > int equal",
			key:      10,
			value:    10,
			expected: false,
		},
		{
			name:     "int64 > int64 true",
			key:      int64(100),
			value:    int64(50),
			expected: true,
		},
		{
			name:     "int > float64 true",
			key:      10,
			value:    5.5,
			expected: true,
		},
		{
			name:     "int > string number true",
			key:      10,
			value:    "5",
			expected: true,
		},
		// Float key comparisons
		{
			name:     "float > float true",
			key:      10.5,
			value:    5.5,
			expected: true,
		},
		{
			name:     "float > float false",
			key:      5.5,
			value:    10.5,
			expected: false,
		},
		{
			name:     "float > int true",
			key:      10.5,
			value:    5,
			expected: true,
		},
		{
			name:     "float > string number true",
			key:      10.5,
			value:    "5.5",
			expected: true,
		},
		// String key comparisons
		{
			name:     "string number > string number true",
			key:      "10",
			value:    "5",
			expected: true,
		},
		{
			name:     "string number > int true",
			key:      "10",
			value:    5,
			expected: true,
		},
		// Resource quantity comparisons
		{
			name:     "resource quantity > quantity true",
			key:      "1Gi",
			value:    "500Mi",
			expected: true,
		},
		{
			name:     "resource quantity > quantity false",
			key:      "256Mi",
			value:    "1Gi",
			expected: false,
		},
		// Semver comparisons
		{
			name:     "semver > semver true",
			key:      "2.0.0",
			value:    "1.0.0",
			expected: true,
		},
		{
			name:     "semver > semver false",
			key:      "1.0.0",
			value:    "2.0.0",
			expected: false,
		},
		// Unsupported types
		{
			name:     "unsupported type",
			key:      true,
			value:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_Evaluate_GreaterThanOrEquals(t *testing.T) {
	log := logr.Discard()
	handler := NewNumericOperatorHandler(log, nil, kyvernov1.ConditionOperators["GreaterThanOrEquals"])

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "int >= int true greater",
			key:      10,
			value:    5,
			expected: true,
		},
		{
			name:     "int >= int true equal",
			key:      10,
			value:    10,
			expected: true,
		},
		{
			name:     "int >= int false",
			key:      5,
			value:    10,
			expected: false,
		},
		{
			name:     "float >= float true",
			key:      10.5,
			value:    10.5,
			expected: true,
		},
		{
			name:     "string number >= string number true",
			key:      "10",
			value:    "10",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_Evaluate_LessThan(t *testing.T) {
	log := logr.Discard()
	handler := NewNumericOperatorHandler(log, nil, kyvernov1.ConditionOperators["LessThan"])

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "int < int true",
			key:      5,
			value:    10,
			expected: true,
		},
		{
			name:     "int < int false",
			key:      10,
			value:    5,
			expected: false,
		},
		{
			name:     "int < int equal",
			key:      10,
			value:    10,
			expected: false,
		},
		{
			name:     "float < float true",
			key:      5.5,
			value:    10.5,
			expected: true,
		},
		{
			name:     "string number < string number true",
			key:      "5",
			value:    "10",
			expected: true,
		},
		{
			name:     "resource quantity < quantity true",
			key:      "256Mi",
			value:    "1Gi",
			expected: true,
		},
		{
			name:     "semver < semver true",
			key:      "1.0.0",
			value:    "2.0.0",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_Evaluate_LessThanOrEquals(t *testing.T) {
	log := logr.Discard()
	handler := NewNumericOperatorHandler(log, nil, kyvernov1.ConditionOperators["LessThanOrEquals"])

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "int <= int true less",
			key:      5,
			value:    10,
			expected: true,
		},
		{
			name:     "int <= int true equal",
			key:      10,
			value:    10,
			expected: true,
		},
		{
			name:     "int <= int false",
			key:      10,
			value:    5,
			expected: false,
		},
		{
			name:     "float <= float true",
			key:      5.5,
			value:    5.5,
			expected: true,
		},
		{
			name:     "string number <= string number true",
			key:      "10",
			value:    "10",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_ValidateValueWithIntPattern(t *testing.T) {
	log := logr.Discard()
	handler := NumericOperatorHandler{
		log:       log,
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	tests := []struct {
		name     string
		key      int64
		value    interface{}
		expected bool
	}{
		{
			name:     "int64 > int",
			key:      int64(10),
			value:    5,
			expected: true,
		},
		{
			name:     "int64 > int64",
			key:      int64(10),
			value:    int64(5),
			expected: true,
		},
		{
			name:     "int64 > float64",
			key:      int64(10),
			value:    5.5,
			expected: true,
		},
		{
			name:     "int64 > string float",
			key:      int64(10),
			value:    "5.5",
			expected: true,
		},
		{
			name:     "int64 > string int",
			key:      int64(10),
			value:    "5",
			expected: true,
		},
		{
			name:     "int64 with invalid string",
			key:      int64(10),
			value:    "abc",
			expected: false,
		},
		{
			name:     "int64 with wrong type",
			key:      int64(10),
			value:    true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateValueWithIntPattern(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_ValidateValueWithFloatPattern(t *testing.T) {
	log := logr.Discard()
	handler := NumericOperatorHandler{
		log:       log,
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	tests := []struct {
		name     string
		key      float64
		value    interface{}
		expected bool
	}{
		{
			name:     "float > int",
			key:      10.5,
			value:    5,
			expected: true,
		},
		{
			name:     "float > int64",
			key:      10.5,
			value:    int64(5),
			expected: true,
		},
		{
			name:     "float > float64",
			key:      10.5,
			value:    5.5,
			expected: true,
		},
		{
			name:     "float > string float",
			key:      10.5,
			value:    "5.5",
			expected: true,
		},
		{
			name:     "float > string int",
			key:      10.5,
			value:    "5",
			expected: true,
		},
		{
			name:     "float with invalid string",
			key:      10.5,
			value:    "abc",
			expected: false,
		},
		{
			name:     "float with wrong type",
			key:      10.5,
			value:    true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateValueWithFloatPattern(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_ValidateValueWithStringPattern(t *testing.T) {
	log := logr.Discard()
	handler := NumericOperatorHandler{
		log:       log,
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected bool
	}{
		{
			name:     "string float > int",
			key:      "10.5",
			value:    5,
			expected: true,
		},
		{
			name:     "string int > int",
			key:      "10",
			value:    5,
			expected: true,
		},
		{
			name:     "resource quantity comparison",
			key:      "1Gi",
			value:    "512Mi",
			expected: true,
		},
		{
			name:     "duration comparison",
			key:      "2h",
			value:    "1h",
			expected: true,
		},
		{
			name:     "semver comparison",
			key:      "2.0.0",
			value:    "1.0.0",
			expected: true,
		},
		{
			name:     "invalid string key",
			key:      "abc",
			value:    5,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.validateValueWithStringPattern(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseQuantity(t *testing.T) {
	tests := []struct {
		name      string
		key       interface{}
		value     interface{}
		expectErr bool
	}{
		{
			name:      "valid quantities",
			key:       "100Mi",
			value:     "200Mi",
			expectErr: false,
		},
		{
			name:      "key not quantity",
			key:       123,
			value:     "200Mi",
			expectErr: true,
		},
		{
			name:      "value not quantity",
			key:       "100Mi",
			value:     123,
			expectErr: true,
		},
		{
			name:      "invalid key string",
			key:       "invalid",
			value:     "200Mi",
			expectErr: true,
		},
		{
			name:      "invalid value string",
			key:       "100Mi",
			value:     "invalid",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseQuantity(tt.key, tt.value)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCompareByCondition(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name      string
		key       float64
		value     float64
		condition kyvernov1.ConditionOperator
		expected  bool
	}{
		{
			name:      "greater than or equals - true",
			key:       10,
			value:     5,
			condition: kyvernov1.ConditionOperators["GreaterThanOrEquals"],
			expected:  true,
		},
		{
			name:      "greater than or equals - equal",
			key:       10,
			value:     10,
			condition: kyvernov1.ConditionOperators["GreaterThanOrEquals"],
			expected:  true,
		},
		{
			name:      "greater than - true",
			key:       10,
			value:     5,
			condition: kyvernov1.ConditionOperators["GreaterThan"],
			expected:  true,
		},
		{
			name:      "greater than - equal",
			key:       10,
			value:     10,
			condition: kyvernov1.ConditionOperators["GreaterThan"],
			expected:  false,
		},
		{
			name:      "less than or equals - true",
			key:       5,
			value:     10,
			condition: kyvernov1.ConditionOperators["LessThanOrEquals"],
			expected:  true,
		},
		{
			name:      "less than or equals - equal",
			key:       10,
			value:     10,
			condition: kyvernov1.ConditionOperators["LessThanOrEquals"],
			expected:  true,
		},
		{
			name:      "less than - true",
			key:       5,
			value:     10,
			condition: kyvernov1.ConditionOperators["LessThan"],
			expected:  true,
		},
		{
			name:      "less than - equal",
			key:       10,
			value:     10,
			condition: kyvernov1.ConditionOperators["LessThan"],
			expected:  false,
		},
		{
			name:      "invalid operator",
			key:       10,
			value:     10,
			condition: kyvernov1.ConditionOperator("Invalid"),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareByCondition(tt.key, tt.value, tt.condition, log)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_UnreachableMethods(t *testing.T) {
	log := logr.Discard()
	handler := NumericOperatorHandler{log: log}

	// These methods should always return false as they're unreachable
	assert.False(t, handler.validateValueWithBoolPattern(true, true))
	assert.False(t, handler.validateValueWithMapPattern(map[string]interface{}{}, nil))
	assert.False(t, handler.validateValueWithSlicePattern([]interface{}{}, nil))
}
