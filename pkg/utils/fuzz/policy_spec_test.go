package fuzz

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func TestCreateRules_Break(t *testing.T) {
	// Provide exactly enough bytes for ff.GetInt() to return a non-zero value,
	// but not enough bytes for the subsequent ff.GetBytes() to succeed.
	// This will hit the `break` statement we added.
	// GetInt() in go-fuzz-headers reads 4 bytes.
	// The bytes [0x05, 0x00, 0x00, 0x00] correspond to integer 5, so noOfRules%100 = 5.
	// Since there are only 4 bytes, ff.GetBytes() will fail immediately, hitting the break.

	data := []byte{0x05, 0x00, 0x00, 0x00}
	ff := fuzz.NewConsumer(data)

	// This should not panic or leak goroutines, and should return immediately.
	rules := createRules(ff)

	if len(rules) != 0 {
		t.Errorf("Expected 0 rules, got %d", len(rules))
	}
}
