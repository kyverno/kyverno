package mutate

import (
	"testing"

	"gotest.tools/assert"
)

func TestRemoveAnchor_ConditionAnchor(t *testing.T) {
	assert.Equal(t, removeAnchor("(abc)"), "abc")
}

func TestRemoveAnchor_ExistanceAnchor(t *testing.T) {
	assert.Equal(t, removeAnchor("^(abc)"), "abc")
}

func TestRemoveAnchor_EmptyExistanceAnchor(t *testing.T) {
	assert.Equal(t, removeAnchor("^()"), "")
}
