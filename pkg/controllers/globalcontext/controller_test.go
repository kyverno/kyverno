package globalcontext

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConstants verifies the controller configuration constants are set correctly
func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{
			name:     "Workers constant",
			got:      Workers,
			expected: 1,
		},
		{
			name:     "ControllerName constant",
			got:      ControllerName,
			expected: "global-context",
		},
		{
			name:     "maxRetries constant",
			got:      maxRetries,
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.got)
		})
	}
}
