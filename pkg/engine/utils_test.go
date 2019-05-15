package engine

import (
	"testing"
)

func assertEqDataImpl(t *testing.T, expected, actual []byte, formatModifier string) {
	if len(expected) != len(actual) {
		t.Errorf("len(expected) != len(actual): %d != %d\n1:"+formatModifier+"\n2:"+formatModifier, len(expected), len(actual), expected, actual)
		return
	}

	for idx, val := range actual {
		if val != expected[idx] {
			t.Errorf("Slices not equal at index %d:\n1:"+formatModifier+"\n2:"+formatModifier, idx, expected, actual)
		}
	}
}

func assertEqData(t *testing.T, expected, actual []byte) {
	assertEqDataImpl(t, expected, actual, "%x")
}

func assertEqStringAndData(t *testing.T, str string, data []byte) {
	assertEqDataImpl(t, []byte(str), data, "%s")
}
