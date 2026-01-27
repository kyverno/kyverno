package operator

import (
	"testing"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

// compareByCondition Tests
func Test_compareByCondition_greater_than(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		key      float64
		value    float64
		expected bool
	}{
		{
			name:     "pod_replicas_5_greater_than_3",
			key:      5.0,
			value:    3.0,
			expected: true,
		},
		{
			name:     "pod_replicas_3_not_greater_than_5",
			key:      3.0,
			value:    5.0,
			expected: false,
		},
		{
			name:     "equal_values_not_greater_than",
			key:      10.0,
			value:    10.0,
			expected: false,
		},
		{
			name:     "negative_values_minus_1_greater_than_minus_5",
			key:      -1.0,
			value:    -5.0,
			expected: true,
		},
		{
			name:     "zero_greater_than_negative",
			key:      0.0,
			value:    -1.0,
			expected: true,
		},
		{
			name:     "decimal_precision_2_5_greater_than_2_4",
			key:      2.5,
			value:    2.4,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := compareByCondition(tt.key, tt.value, kyvernov1.ConditionOperators["GreaterThan"], logr.Discard())
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_compareByCondition_greater_than_or_equals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		key      float64
		value    float64
		expected bool
	}{
		{
			name:     "memory_limit_1024_gte_512",
			key:      1024.0,
			value:    512.0,
			expected: true,
		},
		{
			name:     "exact_equality_passes",
			key:      100.0,
			value:    100.0,
			expected: true,
		},
		{
			name:     "smaller_value_fails",
			key:      50.0,
			value:    100.0,
			expected: false,
		},
		{
			name:     "zero_gte_zero",
			key:      0.0,
			value:    0.0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := compareByCondition(tt.key, tt.value, kyvernov1.ConditionOperators["GreaterThanOrEquals"], logr.Discard())
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_compareByCondition_less_than(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		key      float64
		value    float64
		expected bool
	}{
		{
			name:     "replica_count_2_less_than_5",
			key:      2.0,
			value:    5.0,
			expected: true,
		},
		{
			name:     "larger_value_fails",
			key:      10.0,
			value:    5.0,
			expected: false,
		},
		{
			name:     "equal_values_not_less_than",
			key:      7.0,
			value:    7.0,
			expected: false,
		},
		{
			name:     "negative_minus_10_less_than_minus_5",
			key:      -10.0,
			value:    -5.0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := compareByCondition(tt.key, tt.value, kyvernov1.ConditionOperators["LessThan"], logr.Discard())
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_compareByCondition_less_than_or_equals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		key      float64
		value    float64
		expected bool
	}{
		{
			name:     "cpu_request_100_lte_200",
			key:      100.0,
			value:    200.0,
			expected: true,
		},
		{
			name:     "exact_equality_passes",
			key:      64.0,
			value:    64.0,
			expected: true,
		},
		{
			name:     "larger_value_fails",
			key:      150.0,
			value:    100.0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := compareByCondition(tt.key, tt.value, kyvernov1.ConditionOperators["LessThanOrEquals"], logr.Discard())
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_compareByCondition_invalid_operator(t *testing.T) {
	t.Parallel()

	// When an invalid operator is provided (e.g., typo in policy YAML),
	// the function should safely return false rather than panic
	result := compareByCondition(10.0, 5.0, kyvernov1.ConditionOperator("InvalidOp"), logr.Discard())
	assert.False(t, result, "invalid operator should return false")
}

// NumericOperatorHandler.Evaluate Tests
// The Evaluate method is the main entry point for numeric comparisons in policies.
// It handles type coercion between int, int64, float64, and string representations.
func TestNumericOperatorHandler_Evaluate_int_key(t *testing.T) {
	t.Parallel()

	handler := NumericOperatorHandler{
		ctx:       nil,
		log:       logr.Discard(),
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "int_key_int_value_10_gt_5",
			key:      10,
			value:    5,
			expected: true,
		},
		{
			name:     "int_key_int64_value_100_gt_50",
			key:      100,
			value:    int64(50),
			expected: true,
		},
		{
			name:     "int_key_float64_value_25_gt_20_5",
			key:      25,
			value:    20.5,
			expected: true,
		},
		{
			name:     "int_key_string_value_30_gt_25",
			key:      30,
			value:    "25",
			expected: true,
		},
		{
			name:     "int_key_string_float_value",
			key:      10,
			value:    "5.5",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_Evaluate_int64_key(t *testing.T) {
	t.Parallel()

	handler := NumericOperatorHandler{
		ctx:       nil,
		log:       logr.Discard(),
		condition: kyvernov1.ConditionOperators["LessThan"],
	}

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "int64_key_less_than_int_value",
			key:      int64(5),
			value:    10,
			expected: true,
		},
		{
			name:     "int64_key_less_than_int64_value",
			key:      int64(20),
			value:    int64(30),
			expected: true,
		},
		{
			name:     "int64_key_not_less_than_smaller_value",
			key:      int64(100),
			value:    50,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_Evaluate_float64_key(t *testing.T) {
	t.Parallel()

	handler := NumericOperatorHandler{
		ctx:       nil,
		log:       logr.Discard(),
		condition: kyvernov1.ConditionOperators["GreaterThanOrEquals"],
	}

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "float64_key_gte_int_value",
			key:      10.5,
			value:    10,
			expected: true,
		},
		{
			name:     "float64_key_gte_float64_value_equal",
			key:      3.14,
			value:    3.14,
			expected: true,
		},
		{
			name:     "float64_key_gte_string_value",
			key:      5.5,
			value:    "4.5",
			expected: true,
		},
		{
			name:     "float64_key_less_than_value_fails",
			key:      2.5,
			value:    5.0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericOperatorHandler_Evaluate_string_key_with_numeric_values(t *testing.T) {
	t.Parallel()

	handler := NumericOperatorHandler{
		ctx:       nil,
		log:       logr.Discard(),
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "string_key_parsed_as_float_gt_int",
			key:      "10.5",
			value:    10,
			expected: true,
		},
		{
			name:     "string_key_parsed_as_int_gt_int",
			key:      "100",
			value:    50,
			expected: true,
		},
		{
			name:     "string_key_parsed_as_float_gt_string",
			key:      "25.5",
			value:    "20.0",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Resource Quantity Comparison Tests
func TestNumericOperatorHandler_Evaluate_resource_quantities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition kyvernov1.ConditionOperator
		key       interface{}
		value     interface{}
		expected  bool
	}{
		{
			name:      "cpu_200m_greater_than_100m",
			condition: kyvernov1.ConditionOperators["GreaterThan"],
			key:       "200m",
			value:     "100m",
			expected:  true,
		},
		{
			name:      "cpu_100m_not_greater_than_200m",
			condition: kyvernov1.ConditionOperators["GreaterThan"],
			key:       "100m",
			value:     "200m",
			expected:  false,
		},
		{
			name:      "memory_1Gi_greater_than_500Mi",
			condition: kyvernov1.ConditionOperators["GreaterThan"],
			key:       "1Gi",
			value:     "500Mi",
			expected:  true,
		},
		{
			name:      "memory_256Mi_less_than_1Gi",
			condition: kyvernov1.ConditionOperators["LessThan"],
			key:       "256Mi",
			value:     "1Gi",
			expected:  true,
		},
		{
			name:      "cpu_500m_gte_500m",
			condition: kyvernov1.ConditionOperators["GreaterThanOrEquals"],
			key:       "500m",
			value:     "500m",
			expected:  true,
		},
		{
			name:      "memory_2Gi_lte_2Gi",
			condition: kyvernov1.ConditionOperators["LessThanOrEquals"],
			key:       "2Gi",
			value:     "2Gi",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := NumericOperatorHandler{
				ctx:       nil,
				log:       logr.Discard(),
				condition: tt.condition,
			}
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Semantic Version Comparison Tests
func TestNumericOperatorHandler_Evaluate_semver_versions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition kyvernov1.ConditionOperator
		key       interface{}
		value     interface{}
		expected  bool
	}{
		{
			name:      "version_1_2_0_greater_than_1_1_0",
			condition: kyvernov1.ConditionOperators["GreaterThan"],
			key:       "1.2.0",
			value:     "1.1.0",
			expected:  true,
		},
		{
			name:      "version_1_0_0_less_than_2_0_0",
			condition: kyvernov1.ConditionOperators["LessThan"],
			key:       "1.0.0",
			value:     "2.0.0",
			expected:  true,
		},
		{
			name:      "version_1_5_0_gte_1_5_0",
			condition: kyvernov1.ConditionOperators["GreaterThanOrEquals"],
			key:       "1.5.0",
			value:     "1.5.0",
			expected:  true,
		},
		{
			name:      "version_2_0_0_not_less_than_1_9_0",
			condition: kyvernov1.ConditionOperators["LessThan"],
			key:       "2.0.0",
			value:     "1.9.0",
			expected:  false,
		},
		{
			name:      "version_0_9_0_lte_1_0_0",
			condition: kyvernov1.ConditionOperators["LessThanOrEquals"],
			key:       "0.9.0",
			value:     "1.0.0",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := NumericOperatorHandler{
				ctx:       nil,
				log:       logr.Discard(),
				condition: tt.condition,
			}
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test compareVersionByCondition directly for semver edge cases like:
// - Patch version bumps (1.2.2 -> 1.2.3)
// - Pre-release versions (1.0.0-alpha < 1.0.0)
// - Major version boundaries (1.x.x vs 2.x.x)

func Test_compareVersionByCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		keyStr    string
		valueStr  string
		condition kyvernov1.ConditionOperator
		expected  bool
	}{
		{
			name:      "patch_version_gt",
			keyStr:    "1.2.3",
			valueStr:  "1.2.2",
			condition: kyvernov1.ConditionOperators["GreaterThan"],
			expected:  true,
		},
		{
			name:      "minor_version_lt",
			keyStr:    "1.1.0",
			valueStr:  "1.2.0",
			condition: kyvernov1.ConditionOperators["LessThan"],
			expected:  true,
		},
		{
			name:      "major_version_gte_equal",
			keyStr:    "2.0.0",
			valueStr:  "2.0.0",
			condition: kyvernov1.ConditionOperators["GreaterThanOrEquals"],
			expected:  true,
		},
		{
			name:      "prerelease_version_lt",
			keyStr:    "1.0.0-alpha",
			valueStr:  "1.0.0",
			condition: kyvernov1.ConditionOperators["LessThan"],
			expected:  true,
		},
		{
			name:      "invalid_operator_returns_false",
			keyStr:    "1.0.0",
			valueStr:  "0.9.0",
			condition: kyvernov1.ConditionOperator("Equal"), // Not valid for version comparison
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			keyVer, _ := semver.Parse(tt.keyStr)
			valueVer, _ := semver.Parse(tt.valueStr)
			result := compareVersionByCondition(keyVer, valueVer, tt.condition, logr.Discard())
			assert.Equal(t, tt.expected, result)
		})
	}
}

// parseQuantity Tests

func Test_parseQuantity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         interface{}
		value       interface{}
		expectError bool
	}{
		{
			name:        "valid_cpu_quantities",
			key:         "100m",
			value:       "200m",
			expectError: false,
		},
		{
			name:        "valid_memory_quantities",
			key:         "1Gi",
			value:       "512Mi",
			expectError: false,
		},
		{
			name:        "key_not_string",
			key:         100,
			value:       "200m",
			expectError: true,
		},
		{
			name:        "value_not_string",
			key:         "100m",
			value:       200,
			expectError: true,
		},
		{
			name:        "invalid_quantity_format",
			key:         "not-a-quantity",
			value:       "200m",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := parseQuantity(tt.key, tt.value)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Unsupported Type Handling Tests

func TestNumericOperatorHandler_Evaluate_unsupported_types(t *testing.T) {
	t.Parallel()

	handler := NumericOperatorHandler{
		ctx:       nil,
		log:       logr.Discard(),
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "bool_key_unsupported",
			key:      true,
			value:    false,
			expected: false,
		},
		{
			name:     "map_key_unsupported",
			key:      map[string]interface{}{"foo": "bar"},
			value:    "100",
			expected: false,
		},
		{
			name:     "slice_key_unsupported",
			key:      []interface{}{"a", "b"},
			value:    "100",
			expected: false,
		},
		{
			name:     "nil_key_unsupported",
			key:      nil,
			value:    "100",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test validateValueWithIntPattern rejects invalid value types.
// When the key is an int but the value is a bool or unparseable string,
// the comparison should safely return false.

func TestNumericOperatorHandler_validateValueWithIntPattern_invalid_value(t *testing.T) {
	t.Parallel()

	handler := NumericOperatorHandler{
		ctx:       nil,
		log:       logr.Discard(),
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	// Test with unsupported value type (bool)
	result := handler.validateValueWithIntPattern(10, true)
	assert.False(t, result, "bool value type should return false")

	// Test with unparseable string
	result = handler.validateValueWithIntPattern(10, "not-a-number")
	assert.False(t, result, "unparseable string should return false")
}

// Test validateValueWithFloatPattern rejects invalid value types.
// Float keys with bool or unparseable string values should return false.

func TestNumericOperatorHandler_validateValueWithFloatPattern_invalid_value(t *testing.T) {
	t.Parallel()

	handler := NumericOperatorHandler{
		ctx:       nil,
		log:       logr.Discard(),
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	// Test with unsupported value type (bool)
	result := handler.validateValueWithFloatPattern(10.5, true)
	assert.False(t, result, "bool value type should return false")

	// Test with unparseable string
	result = handler.validateValueWithFloatPattern(10.5, "not-a-number")
	assert.False(t, result, "unparseable string should return false")
}

// Stub Method Tests (OperatorHandler Interface Compliance)

func TestNumericOperatorHandler_stub_methods_return_false(t *testing.T) {
	t.Parallel()

	handler := NumericOperatorHandler{
		ctx:       nil,
		log:       logr.Discard(),
		condition: kyvernov1.ConditionOperators["GreaterThan"],
	}

	t.Run("validateValueWithBoolPattern_returns_false", func(t *testing.T) {
		t.Parallel()
		result := handler.validateValueWithBoolPattern(true, "value")
		assert.False(t, result)
	})

	t.Run("validateValueWithMapPattern_returns_false", func(t *testing.T) {
		t.Parallel()
		result := handler.validateValueWithMapPattern(map[string]interface{}{"key": "val"}, "value")
		assert.False(t, result)
	})

	t.Run("validateValueWithSlicePattern_returns_false", func(t *testing.T) {
		t.Parallel()
		result := handler.validateValueWithSlicePattern([]interface{}{"a", "b"}, "value")
		assert.False(t, result)
	})
}
