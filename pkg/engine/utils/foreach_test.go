package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_InvertElements(t *testing.T) {
	elems := []interface{}{"a", "b", "c"}
	elemsInverted := InvertElements(elems)

	assert.Equal(t, "a", elemsInverted[2])
	assert.Equal(t, "b", elemsInverted[1])
	assert.Equal(t, "c", elemsInverted[0])
}
