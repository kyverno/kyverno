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

func TestGetOperatorFromStringPattern_RangeOperator(t *testing.T) {
	/*
		assert.Equal(t, GetOperatorFromStringPattern("[,]"), Range)
		assert.Equal(t, GetOperatorFromStringPattern("(,)"), Range)
		assert.Equal(t, GetOperatorFromStringPattern("(,]"), Range)
		assert.Equal(t, GetOperatorFromStringPattern("[,)"), Range)
	*/

	assert.Equal(t, GetOperatorFromStringPattern("0-1"), Range)
	assert.Equal(t, GetOperatorFromStringPattern("0Mi-1024Mi"), Range)
	assert.Equal(t, GetOperatorFromStringPattern("text1024Mi-2048Mi"), Equal)
	assert.Equal(t, GetOperatorFromStringPattern("test-value"), Equal)
	assert.Equal(t, GetOperatorFromStringPattern("value-*"), Equal)
}
