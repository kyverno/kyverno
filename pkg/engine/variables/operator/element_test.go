package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToElement(t *testing.T) {
	tests := []struct {
		name           string
		value          interface{}
		expectStr      string
		expectComparab bool
	}{
		{name: "string", value: "a", expectStr: "a", expectComparab: true},
		{name: "int", value: 1, expectStr: "1", expectComparab: true},
		{name: "int32", value: int32(1), expectStr: "1", expectComparab: true},
		{name: "int64", value: int64(1), expectStr: "1", expectComparab: true},
		{name: "float32", value: float32(1.5), expectStr: "1.5", expectComparab: true},
		{name: "float64", value: float64(1), expectStr: "1", expectComparab: true},
		{name: "bool", value: true, expectStr: "true", expectComparab: true},
		{name: "map is opaque", value: map[string]interface{}{"a": "b"}, expectComparab: false},
		{name: "slice is opaque", value: []interface{}{"a"}, expectComparab: false},
		{name: "nil is opaque", value: nil, expectComparab: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := toElement(tt.value)
			assert.Equal(t, tt.expectComparab, e.comparable)
			if tt.expectComparab {
				assert.Equal(t, tt.expectStr, e.str)
			}
		})
	}
}

func TestIsUniversalPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		expected bool
	}{
		{"*", true},
		{"**", true},
		{"?", false},
		{"", false},
		{"*a*", false},
		{"prod*", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			assert.Equal(t, tt.expected, isUniversalPattern(tt.pattern))
		})
	}
}

func TestEqualElements(t *testing.T) {
	comparableA := toElement("a")
	comparableA2 := toElement("a")
	comparableB := toElement("b")
	opaque := toElement(map[string]interface{}{"a": "b"})
	opaque2 := toElement([]interface{}{"x"})

	assert.True(t, equalElements(comparableA, comparableA2))
	assert.False(t, equalElements(comparableA, comparableB))
	assert.False(t, equalElements(opaque, comparableA))
	assert.False(t, equalElements(comparableA, opaque))
	// Two opaque elements are never equal, even to themselves -- no
	// fmt.Sprint junk-text matching (e.g. "map[a:b]" == "map[a:b]").
	assert.False(t, equalElements(opaque, opaque))
	assert.False(t, equalElements(opaque, opaque2))
}
