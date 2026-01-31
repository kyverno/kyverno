package operator

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

func TestDurationOperatorHandler_Evaluate_GreaterThan(t *testing.T) {
	log := logr.Discard()
	handler := NewDurationOperatorHandler(log, nil, kyvernov1.ConditionOperators["DurationGreaterThan"])

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		// String duration comparisons
		{
			name:     "duration string > string true",
			key:      "2h",
			value:    "1h",
			expected: true,
		},
		{
			name:     "duration string > string false",
			key:      "1h",
			value:    "2h",
			expected: false,
		},
		{
			name:     "duration string > string equal",
			key:      "1h",
			value:    "1h",
			expected: false,
		},
		{
			name:     "duration 60m > 1h equal",
			key:      "60m",
			value:    "1h",
			expected: false,
		},
		{
			name:     "duration 61m > 1h true",
			key:      "61m",
			value:    "1h",
			expected: true,
		},
		// Int comparisons (treated as seconds)
		{
			name:     "int > int true",
			key:      120,
			value:    60,
			expected: true,
		},
		{
			name:     "int > int false",
			key:      60,
			value:    120,
			expected: false,
		},
		{
			name:     "int64 > int64 true",
			key:      int64(3600),
			value:    int64(1800),
			expected: true,
		},
		{
			name:     "int > string duration true",
			key:      120,
			value:    "1m",
			expected: true,
		},
		// Float comparisons (treated as seconds)
		{
			name:     "float > float true",
			key:      120.5,
			value:    60.5,
			expected: true,
		},
		{
			name:     "float > int true",
			key:      120.5,
			value:    60,
			expected: true,
		},
		{
			name:     "float > string duration true",
			key:      120.0,
			value:    "1m",
			expected: true,
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

func TestDurationOperatorHandler_Evaluate_GreaterThanOrEquals(t *testing.T) {
	log := logr.Discard()
	handler := NewDurationOperatorHandler(log, nil, kyvernov1.ConditionOperators["DurationGreaterThanOrEquals"])

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "duration >= duration true greater",
			key:      "2h",
			value:    "1h",
			expected: true,
		},
		{
			name:     "duration >= duration true equal",
			key:      "1h",
			value:    "60m",
			expected: true,
		},
		{
			name:     "duration >= duration false",
			key:      "30m",
			value:    "1h",
			expected: false,
		},
		{
			name:     "int >= int true equal",
			key:      60,
			value:    60,
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

func TestDurationOperatorHandler_Evaluate_LessThan(t *testing.T) {
	log := logr.Discard()
	handler := NewDurationOperatorHandler(log, nil, kyvernov1.ConditionOperators["DurationLessThan"])

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "duration < duration true",
			key:      "30m",
			value:    "1h",
			expected: true,
		},
		{
			name:     "duration < duration false",
			key:      "2h",
			value:    "1h",
			expected: false,
		},
		{
			name:     "duration < duration equal",
			key:      "1h",
			value:    "60m",
			expected: false,
		},
		{
			name:     "int < int true",
			key:      30,
			value:    60,
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

func TestDurationOperatorHandler_Evaluate_LessThanOrEquals(t *testing.T) {
	log := logr.Discard()
	handler := NewDurationOperatorHandler(log, nil, kyvernov1.ConditionOperators["DurationLessThanOrEquals"])

	tests := []struct {
		name     string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{
			name:     "duration <= duration true less",
			key:      "30m",
			value:    "1h",
			expected: true,
		},
		{
			name:     "duration <= duration true equal",
			key:      "1h",
			value:    "60m",
			expected: true,
		},
		{
			name:     "duration <= duration false",
			key:      "2h",
			value:    "1h",
			expected: false,
		},
		{
			name:     "int <= int true equal",
			key:      60,
			value:    60,
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

func TestDurationOperatorHandler_ValidateValueWithIntPattern(t *testing.T) {
	log := logr.Discard()
	handler := DurationOperatorHandler{
		log:       log,
		condition: kyvernov1.ConditionOperators["DurationGreaterThan"],
	}

	tests := []struct {
		name     string
		key      int64
		value    interface{}
		expected bool
	}{
		{
			name:     "int64 > int",
			key:      int64(120),
			value:    60,
			expected: true,
		},
		{
			name:     "int64 > int64",
			key:      int64(120),
			value:    int64(60),
			expected: true,
		},
		{
			name:     "int64 > float64",
			key:      int64(120),
			value:    60.0,
			expected: true,
		},
		{
			name:     "int64 > string duration",
			key:      int64(120),
			value:    "1m",
			expected: true,
		},
		{
			name:     "int64 with invalid string",
			key:      int64(120),
			value:    "invalid",
			expected: false,
		},
		{
			name:     "int64 with wrong type",
			key:      int64(120),
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

func TestDurationOperatorHandler_ValidateValueWithFloatPattern(t *testing.T) {
	log := logr.Discard()
	handler := DurationOperatorHandler{
		log:       log,
		condition: kyvernov1.ConditionOperators["DurationGreaterThan"],
	}

	tests := []struct {
		name     string
		key      float64
		value    interface{}
		expected bool
	}{
		{
			name:     "float > int",
			key:      120.0,
			value:    60,
			expected: true,
		},
		{
			name:     "float > int64",
			key:      120.0,
			value:    int64(60),
			expected: true,
		},
		{
			name:     "float > float64",
			key:      120.0,
			value:    60.0,
			expected: true,
		},
		{
			name:     "float > string duration",
			key:      120.0,
			value:    "1m",
			expected: true,
		},
		{
			name:     "float with invalid string",
			key:      120.0,
			value:    "invalid",
			expected: false,
		},
		{
			name:     "float with wrong type",
			key:      120.0,
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

func TestDurationOperatorHandler_ValidateValueWithStringPattern(t *testing.T) {
	log := logr.Discard()
	handler := DurationOperatorHandler{
		log:       log,
		condition: kyvernov1.ConditionOperators["DurationGreaterThan"],
	}

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected bool
	}{
		{
			name:     "string duration > int",
			key:      "2m",
			value:    60,
			expected: true,
		},
		{
			name:     "string duration > int64",
			key:      "2m",
			value:    int64(60),
			expected: true,
		},
		{
			name:     "string duration > float64",
			key:      "2m",
			value:    60.0,
			expected: true,
		},
		{
			name:     "string duration > string duration",
			key:      "2h",
			value:    "1h",
			expected: true,
		},
		{
			name:     "invalid key string",
			key:      "invalid",
			value:    60,
			expected: false,
		},
		{
			name:     "string duration with invalid value string",
			key:      "1h",
			value:    "invalid",
			expected: false,
		},
		{
			name:     "string duration with wrong type",
			key:      "1h",
			value:    true,
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

func TestDurationCompareByCondition(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name      string
		key       time.Duration
		value     time.Duration
		condition kyvernov1.ConditionOperator
		expected  bool
	}{
		{
			name:      "DurationGreaterThanOrEquals - true greater",
			key:       2 * time.Hour,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperators["DurationGreaterThanOrEquals"],
			expected:  true,
		},
		{
			name:      "DurationGreaterThanOrEquals - true equal",
			key:       1 * time.Hour,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperators["DurationGreaterThanOrEquals"],
			expected:  true,
		},
		{
			name:      "DurationGreaterThan - true",
			key:       2 * time.Hour,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperators["DurationGreaterThan"],
			expected:  true,
		},
		{
			name:      "DurationGreaterThan - equal false",
			key:       1 * time.Hour,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperators["DurationGreaterThan"],
			expected:  false,
		},
		{
			name:      "DurationLessThanOrEquals - true less",
			key:       30 * time.Minute,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperators["DurationLessThanOrEquals"],
			expected:  true,
		},
		{
			name:      "DurationLessThanOrEquals - true equal",
			key:       1 * time.Hour,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperators["DurationLessThanOrEquals"],
			expected:  true,
		},
		{
			name:      "DurationLessThan - true",
			key:       30 * time.Minute,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperators["DurationLessThan"],
			expected:  true,
		},
		{
			name:      "DurationLessThan - equal false",
			key:       1 * time.Hour,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperators["DurationLessThan"],
			expected:  false,
		},
		{
			name:      "invalid operator",
			key:       1 * time.Hour,
			value:     1 * time.Hour,
			condition: kyvernov1.ConditionOperator("Invalid"),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := durationCompareByCondition(tt.key, tt.value, tt.condition, log)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDurationOperatorHandler_UnreachableMethods(t *testing.T) {
	log := logr.Discard()
	handler := DurationOperatorHandler{log: log}

	// These methods should always return false as they're unreachable
	assert.False(t, handler.validateValueWithBoolPattern(true, true))
	assert.False(t, handler.validateValueWithMapPattern(map[string]interface{}{}, nil))
	assert.False(t, handler.validateValueWithSlicePattern([]interface{}{}, nil))
}
