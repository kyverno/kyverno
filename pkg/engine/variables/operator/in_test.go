package operator

import (
	"testing"
)

func Test_ValidateValueWithStringSetPattern(t *testing.T) {
	//var in InHandler
	//in := NewInHandler(log, ctx, subHandler)
	key := []string{"a", "b", "c"}
	value := []string{"a", "b", "c", "d", "e"}
	if !in.validateValueWithStringSetPattern(key, value) {
		t.Errorf("Key was a subset of value but function returned otherwise")
	}
}
