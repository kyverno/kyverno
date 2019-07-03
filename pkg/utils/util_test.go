package utils

import (
	"testing"

	"gotest.tools/assert"
)

func Test_allEmpty(t *testing.T) {
	var list []string
	var element string
	res := Contains(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyList(t *testing.T) {
	var list []string
	element := "foo"
	res := Contains(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyElement(t *testing.T) {
	list := []string{"foo", "bar"}
	var element string
	res := Contains(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyElementInList(t *testing.T) {
	list := []string{"foo", "bar", ""}
	var element string
	res := Contains(list, element)
	assert.Assert(t, res == true)

	list = []string{"foo", "bar", "bar"}
	element = "bar"
	res = Contains(list, element)
	assert.Assert(t, res == true)
}
