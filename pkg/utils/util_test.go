package utils

import (
	"testing"

	"gotest.tools/assert"
)

func Test_allEmpty(t *testing.T) {
	var list []string
	var element string
	res := ContainsString(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyList(t *testing.T) {
	var list []string
	element := "foo"
	res := ContainsString(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyElement(t *testing.T) {
	list := []string{"foo", "bar"}
	var element string
	res := ContainsString(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyElementInList(t *testing.T) {
	list := []string{"foo", "bar", ""}
	var element string
	res := ContainsString(list, element)
	assert.Assert(t, res == true)

	list = []string{"foo", "bar", "bar"}
	element = "bar"
	res = ContainsString(list, element)
	assert.Assert(t, res == true)
}

func Test_containsNs(t *testing.T) {
	var patterns []string
	var res bool
	patterns = []string{"*"}
	res = ContainsNamepace(patterns, "default")
	assert.Assert(t, res == true)

	patterns = []string{"*", "default"}
	res = ContainsNamepace(patterns, "default")
	assert.Assert(t, res == true)

	patterns = []string{"default2", "default"}
	res = ContainsNamepace(patterns, "default1")
	assert.Assert(t, res == false)

	patterns = []string{"d*"}
	res = ContainsNamepace(patterns, "default")
	assert.Assert(t, res == true)

	patterns = []string{"d*"}
	res = ContainsNamepace(patterns, "test")
	assert.Assert(t, res == false)

	patterns = []string{}
	res = ContainsNamepace(patterns, "test")
	assert.Assert(t, res == false)

}
