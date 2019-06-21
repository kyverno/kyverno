package event

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func TestPositive(t *testing.T) {
	resourceName := "test_resource"
	policy := "test_policy"
	expectedMsg := fmt.Sprintf("Policy %s applied successfully on the resource %s", policy, resourceName)
	msg, err := getEventMsg(SPolicyApply, policy, resourceName)
	assert.NilError(t, err)
	assert.Equal(t, expectedMsg, msg)
}

// passing incorrect args
func TestIncorrectArgs(t *testing.T) {
	resourceName := "test_resource"
	_, err := getEventMsg(SPolicyApply, resourceName)
	assert.Error(t, err, "message expects 2 arguments, but 1 arguments passed")
}
