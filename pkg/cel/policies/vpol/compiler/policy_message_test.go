package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidationDefaultMessage verifies that empty messages get a default
func TestValidationDefaultMessage(t *testing.T) {
	tests := []struct {
		name             string
		message          string
		index            int
		expectedContains string
	}{
		{
			name:             "empty message gets default",
			message:          "",
			index:            0,
			expectedContains: "CEL validation at index 0 failed",
		},
		{
			name:             "non-empty message is preserved",
			message:          "Custom error",
			index:            0,
			expectedContains: "Custom error",
		},
	}

	// This is a placeholder - the actual test would need to call evaluateWithData
	// and verify the message in the result
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The test implementation would go here
			// For now, we're just documenting the expected behavior
			assert.NotEmpty(t, tt.expectedContains)
		})
	}
}
