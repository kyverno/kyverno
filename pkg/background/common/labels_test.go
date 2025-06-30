package common

import (
	"testing"

	"gotest.tools/assert"
)

func TestHashEncodeName(t *testing.T) {
	// Ensure predictable hash values and length
	testCases := []struct {
		name     string
		expected string
	}{
		{
			name:     "test",
			expected: "t6dnbamijr6wlgrp5kqmkwwqcwr36ty3fmfyelgrlvwblmhqbiea",
		},
		{
			name:     "another:value",
			expected: "5m276jyed5ktuzfbjn6g2gqzqyddkrsp4rkvjw2efc5f6ownlieq",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hashEncodeName(tc.name)
			assert.Equal(t, result, tc.expected)
			assert.Equal(t, len(result), 52, "Encoded name should be 52 characters long")
		})
	}
}
