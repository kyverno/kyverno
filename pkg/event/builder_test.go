package event

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func TestPositive(t *testing.T) {
	resourceName := "test_resource"
	expectedMsg := fmt.Sprintf("Policy applied successfully on the resource '%s'", resourceName)
	msg, err := getEventMsg(SPolicyApply, resourceName)
	assert.NilError(t, err)
	assert.Equal(t, expectedMsg, msg)
}

// passing incorrect args
func TestIncorrectArgs(t *testing.T) {
	resourceName := "test_resource"
	_, err := getEventMsg(SPolicyApply, resourceName, "extra_args")
	assert.Error(t, err, "message expects 1 arguments, but 2 arguments passed")
}
