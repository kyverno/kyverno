package operator

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

func Test_NumericOperatorHandler_Evaluate(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		key      interface{}
		value    interface{}
		expected bool
	}{
		{"gt true", "GreaterThan", 10, 5, true},
		{"gt false", "GreaterThan", 5, 10, false},
		{"gt equal", "GreaterThan", 5, 5, false},
		{"ge true", "GreaterThanOrEquals", 10, 10, true},
		{"lt true", "LessThan", 5, 10, true},
		{"le true", "LessThanOrEquals", 5, 5, true},

		{"float gt", "GreaterThan", 10.5, 10.4, true},
		{"float lt", "LessThan", 10.4, 10.5, true},

		{"string numeric gt", "GreaterThan", "100", 50, true},
		{"string numeric lt", "LessThan", "50", "100", true},

		{"semver gt", "GreaterThan", "1.2.3", "1.2.2", true},
		{"semver lt", "LessThan", "1.1.0", "1.2.0", true},
		{"semver ge", "GreaterThanOrEquals", "1.2.0", "1.2.0", true},

		{"resource gt", "GreaterThan", "200Mi", "100Mi", true},
		{"resource lt", "LessThan", "1Gi", "2Gi", true},
		{"resource ge", "GreaterThanOrEquals", "500m", "0.5", true},

		{"unsupported type key", "GreaterThan", true, 5, false},
		{"invalid numeric string", "GreaterThan", "not-a-number", 5, false},
		{"nil key", "GreaterThan", nil, 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opSymbol := kyvernov1.ConditionOperators[tt.operator]

			handler := NewNumericOperatorHandler(logr.Discard(), nil, opSymbol)

			result := handler.Evaluate(tt.key, tt.value)
			assert.Equal(t, tt.expected, result, "Failure in test case: %s", tt.name)
		})
	}
}
