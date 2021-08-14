package event

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
)

func TestPositive(t *testing.T) {
	resourceName := "test_resource"
	ruleName := "test_rule"
	expectedMsg := fmt.Sprintf("Rule(s) '%s' failed to apply on resource %s", ruleName, resourceName)
	msg, err := getEventMsg(FPolicyApply, ruleName, resourceName)
	assert.NilError(t, err)
	assert.Equal(t, expectedMsg, msg)
}

// passing incorrect args
func TestIncorrectArgs(t *testing.T) {
	resourceName := "test_resource"
	_, err := getEventMsg(FPolicyApply, resourceName, "extra_args1", "extra_args2")
	assert.Error(t, err, "message expects 2 arguments, but 3 arguments passed")
}
