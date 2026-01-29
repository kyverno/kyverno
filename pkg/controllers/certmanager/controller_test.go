package certmanager

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
			name:     "Workers constant",
			got:      Workers,
			expected: 1,
		},
		{
			name:     "ControllerName constant",
			got:      ControllerName,
			expected: "certmanager-controller",
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

func TestControllerNameValue(t *testing.T) {
	assert.NotEmpty(t, ControllerName)
	assert.Equal(t, "certmanager-controller", ControllerName)
}

func TestWorkersValue(t *testing.T) {
	assert.Equal(t, 1, Workers)
	assert.Greater(t, Workers, 0)
}

func TestMaxRetriesValue(t *testing.T) {
	assert.Equal(t, 10, maxRetries)
	assert.Greater(t, maxRetries, 0)
}
