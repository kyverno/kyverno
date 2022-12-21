package data

import (
	"testing"

	"gotest.tools/assert"
)

func Test_OriginalMapMustNotBeChanged(t *testing.T) {
	// no variables
	originalMap := map[string]interface{}{
		"rsc": 3711,
		"r":   2138,
		"gri": 1908,
		"adg": 912,
	}

	mapCopy := CopyMap(originalMap)
	mapCopy["r"] = 1

	assert.Equal(t, originalMap["r"], 2138)
}
