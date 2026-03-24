package operator

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

// CreateOperatorHandler tests

func TestCreateOperatorHandler_Equal(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name string
		op   kyvernov1.ConditionOperator
	}{
		{
			name: "Equal operator",
			op:   kyvernov1.ConditionOperators["Equal"],
		},
		{
			name: "Equals operator",
			op:   kyvernov1.ConditionOperators["Equals"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CreateOperatorHandler(log, nil, tt.op)
			assert.NotNil(t, handler)
			_, ok := handler.(EqualHandler)
			assert.True(t, ok)
		})
	}
}

func TestCreateOperatorHandler_NotEqual(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name string
		op   kyvernov1.ConditionOperator
	}{
		{
			name: "NotEqual operator",
			op:   kyvernov1.ConditionOperators["NotEqual"],
		},
		{
			name: "NotEquals operator",
			op:   kyvernov1.ConditionOperators["NotEquals"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CreateOperatorHandler(log, nil, tt.op)
			assert.NotNil(t, handler)
			_, ok := handler.(NotEqualHandler)
			assert.True(t, ok)
		})
	}
}

func TestCreateOperatorHandler_InOperators(t *testing.T) {
	log := logr.Discard()

	t.Run("In operator", func(t *testing.T) {
		handler := CreateOperatorHandler(log, nil, kyvernov1.ConditionOperators["In"])
		assert.NotNil(t, handler)
		_, ok := handler.(InHandler)
		assert.True(t, ok)
	})

	t.Run("AnyIn operator", func(t *testing.T) {
		handler := CreateOperatorHandler(log, nil, kyvernov1.ConditionOperators["AnyIn"])
		assert.NotNil(t, handler)
		_, ok := handler.(AnyInHandler)
		assert.True(t, ok)
	})

	t.Run("AllIn operator", func(t *testing.T) {
		handler := CreateOperatorHandler(log, nil, kyvernov1.ConditionOperators["AllIn"])
		assert.NotNil(t, handler)
		_, ok := handler.(AllInHandler)
		assert.True(t, ok)
	})

	t.Run("NotIn operator", func(t *testing.T) {
		handler := CreateOperatorHandler(log, nil, kyvernov1.ConditionOperators["NotIn"])
		assert.NotNil(t, handler)
		_, ok := handler.(NotInHandler)
		assert.True(t, ok)
	})

	t.Run("AnyNotIn operator", func(t *testing.T) {
		handler := CreateOperatorHandler(log, nil, kyvernov1.ConditionOperators["AnyNotIn"])
		assert.NotNil(t, handler)
		_, ok := handler.(AnyNotInHandler)
		assert.True(t, ok)
	})

	t.Run("AllNotIn operator", func(t *testing.T) {
		handler := CreateOperatorHandler(log, nil, kyvernov1.ConditionOperators["AllNotIn"])
		assert.NotNil(t, handler)
		_, ok := handler.(AllNotInHandler)
		assert.True(t, ok)
	})
}

func TestCreateOperatorHandler_NumericOperators(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name string
		op   kyvernov1.ConditionOperator
	}{
		{
			name: "GreaterThanOrEquals operator",
			op:   kyvernov1.ConditionOperators["GreaterThanOrEquals"],
		},
		{
			name: "GreaterThan operator",
			op:   kyvernov1.ConditionOperators["GreaterThan"],
		},
		{
			name: "LessThanOrEquals operator",
			op:   kyvernov1.ConditionOperators["LessThanOrEquals"],
		},
		{
			name: "LessThan operator",
			op:   kyvernov1.ConditionOperators["LessThan"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CreateOperatorHandler(log, nil, tt.op)
			assert.NotNil(t, handler)
			_, ok := handler.(NumericOperatorHandler)
			assert.True(t, ok)
		})
	}
}

func TestCreateOperatorHandler_DurationOperators(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name string
		op   kyvernov1.ConditionOperator
	}{
		{
			name: "DurationGreaterThanOrEquals operator",
			op:   kyvernov1.ConditionOperators["DurationGreaterThanOrEquals"],
		},
		{
			name: "DurationGreaterThan operator",
			op:   kyvernov1.ConditionOperators["DurationGreaterThan"],
		},
		{
			name: "DurationLessThanOrEquals operator",
			op:   kyvernov1.ConditionOperators["DurationLessThanOrEquals"],
		},
		{
			name: "DurationLessThan operator",
			op:   kyvernov1.ConditionOperators["DurationLessThan"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CreateOperatorHandler(log, nil, tt.op)
			assert.NotNil(t, handler)
			_, ok := handler.(DurationOperatorHandler)
			assert.True(t, ok)
		})
	}
}

func TestCreateOperatorHandler_UnsupportedOperator(t *testing.T) {
	log := logr.Discard()
	handler := CreateOperatorHandler(log, nil, kyvernov1.ConditionOperator("InvalidOp"))
	assert.Nil(t, handler)
}

func TestCreateOperatorHandler_CaseInsensitive(t *testing.T) {
	log := logr.Discard()

	tests := []struct {
		name string
		op   kyvernov1.ConditionOperator
	}{
		{
			name: "uppercase EQUALS",
			op:   kyvernov1.ConditionOperator("EQUALS"),
		},
		{
			name: "mixed case Equals",
			op:   kyvernov1.ConditionOperator("Equals"),
		},
		{
			name: "lowercase equals",
			op:   kyvernov1.ConditionOperator("equals"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CreateOperatorHandler(log, nil, tt.op)
			assert.NotNil(t, handler)
			_, ok := handler.(EqualHandler)
			assert.True(t, ok)
		})
	}
}

// parseDuration tests

func TestParseDuration_BothStringDurations(t *testing.T) {
	tests := []struct {
		name          string
		key           interface{}
		value         interface{}
		expectedKey   time.Duration
		expectedValue time.Duration
	}{
		{
			name:          "hours",
			key:           "2h",
			value:         "1h",
			expectedKey:   2 * time.Hour,
			expectedValue: 1 * time.Hour,
		},
		{
			name:          "mixed units",
			key:           "90m",
			value:         "1h30m",
			expectedKey:   90 * time.Minute,
			expectedValue: 90 * time.Minute,
		},
		{
			name:          "seconds",
			key:           "30s",
			value:         "45s",
			expectedKey:   30 * time.Second,
			expectedValue: 45 * time.Second,
		},
		{
			name:          "explicit zero duration 0s",
			key:           "0s",
			value:         "1h",
			expectedKey:   0,
			expectedValue: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyDur, valueDur, err := parseDuration(tt.key, tt.value)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedKey, *keyDur)
			assert.Equal(t, tt.expectedValue, *valueDur)
		})
	}
}

func TestParseDuration_StringDurationWithNumeric(t *testing.T) {
	tests := []struct {
		name          string
		key           interface{}
		value         interface{}
		expectedKey   time.Duration
		expectedValue time.Duration
	}{
		{
			name:          "string key with int value",
			key:           "2m",
			value:         60,
			expectedKey:   2 * time.Minute,
			expectedValue: 60 * time.Second,
		},
		{
			name:          "string key with int64 value",
			key:           "1h",
			value:         int64(120),
			expectedKey:   1 * time.Hour,
			expectedValue: 120 * time.Second,
		},
		{
			name:          "string key with float64 value",
			key:           "30s",
			value:         10.0,
			expectedKey:   30 * time.Second,
			expectedValue: 10 * time.Second,
		},
		{
			name:          "int key with string value",
			key:           120,
			value:         "2m",
			expectedKey:   120 * time.Second,
			expectedValue: 2 * time.Minute,
		},
		{
			name:          "int64 key with string value",
			key:           int64(3600),
			value:         "30m",
			expectedKey:   3600 * time.Second,
			expectedValue: 30 * time.Minute,
		},
		{
			name:          "float64 key with string value",
			key:           60.0,
			value:         "1m",
			expectedKey:   60 * time.Second,
			expectedValue: 1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyDur, valueDur, err := parseDuration(tt.key, tt.value)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedKey, *keyDur)
			assert.Equal(t, tt.expectedValue, *valueDur)
		})
	}
}

func TestParseDuration_NeitherIsDuration(t *testing.T) {
	tests := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{
			name:  "both non-duration strings",
			key:   "hello",
			value: "world",
		},
		{
			name:  "both ints without duration context",
			key:   100,
			value: 200,
		},
		{
			name:  "zero string values",
			key:   "0",
			value: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyDur, valueDur, err := parseDuration(tt.key, tt.value)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "neither value is a duration")
			assert.Nil(t, keyDur)
			assert.Nil(t, valueDur)
		})
	}
}

func TestParseDuration_InvalidNonDurationFallback(t *testing.T) {
	tests := []struct {
		name  string
		key   interface{}
		value interface{}
	}{
		{
			name:  "string duration key with bool value",
			key:   "1h",
			value: true,
		},
		{
			name:  "bool key with string duration value",
			key:   true,
			value: "1h",
		},
		{
			name:  "string duration key with unsupported type value",
			key:   "1h",
			value: struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseDuration(tt.key, tt.value)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "no valid duration value")
		})
	}
}
