package certmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConstants verifies the controller configuration constants are set correctly
func TestConstants(t *testing.T) {
	t.Run("Workers", func(t *testing.T) {
		assert.Equal(t, 1, Workers)
		assert.Greater(t, Workers, 0)
	})

	t.Run("ControllerName", func(t *testing.T) {
		assert.Equal(t, "certmanager-controller", ControllerName)
		assert.NotEmpty(t, ControllerName)
	})

	t.Run("maxRetries", func(t *testing.T) {
		assert.Equal(t, maxRetries, 10)
		assert.Greater(t, maxRetries, 0)
	})
}
