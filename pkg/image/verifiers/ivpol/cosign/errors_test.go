package cosign

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Locks in the Setup/IsSetup contract that pkg/cel/libs/imageverify/impl.go
// relies on to tell an infrastructure/setup failure (surface as an evaluation
// error so failurePolicy applies) from a genuine "not verified" result.
func TestSetupError(t *testing.T) {
	t.Run("Setup(nil) is nil", func(t *testing.T) {
		assert.NoError(t, Setup(nil))
	})

	t.Run("IsSetup is true for a wrapped setup error", func(t *testing.T) {
		err := Setup(errors.New("tuf unreachable"))
		assert.True(t, IsSetup(err))
		assert.EqualError(t, err, "tuf unreachable")
	})

	t.Run("IsSetup is false for a plain error", func(t *testing.T) {
		assert.False(t, IsSetup(errors.New("no matching attestations")))
		assert.False(t, IsSetup(nil))
	})

	t.Run("IsSetup follows the wrap chain", func(t *testing.T) {
		err := fmt.Errorf("outer: %w", Setup(errors.New("inner")))
		assert.True(t, IsSetup(err))
	})

	t.Run("Unwrap returns the cause", func(t *testing.T) {
		cause := errors.New("cause")
		var se *SetupError
		assert.True(t, errors.As(Setup(cause), &se))
		assert.Equal(t, cause, se.Unwrap())
	})
}
