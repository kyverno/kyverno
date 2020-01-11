package operator

import (
	"testing"
	"gotest.tools/assert"
)
func TestGetOperatorFromStringPattern_OneChar(t *testing.T) {
	assert.Equal(t, GetOperatorFromStringPattern("f"), Equal)
}

func TestGetOperatorFromStringPattern_EmptyString(t *testing.T) {
	assert.Equal(t, GetOperatorFromStringPattern(""), Equal)
}

func TestGetOperatorFromStringPattern_OnlyOperator(t *testing.T) {
	assert.Equal(t, GetOperatorFromStringPattern(">="), MoreEqual)
}
