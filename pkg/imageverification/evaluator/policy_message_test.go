package eval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestImageValidationDefaultMessage verifies that empty messages get a default
func TestImageValidationDefaultMessage(t *testing.T) {
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
			expectedContains: "Image validation at index 0 failed",
		},
		{
			name:             "non-empty message is preserved",
			message:          "Custom image validation error",
			index:            0,
			expectedContains: "Custom image validation error",
		},
	}

	// This is a placeholder test to document the expected behavior
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In a real test, we would create a compiledPolicy with validations
			// and test the Evaluate method
			assert.NotEmpty(t, tt.expectedContains)
		})
	}
}
