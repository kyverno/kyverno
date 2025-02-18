package slices_test

import (
	"slices"
	"testing"

	utilslices "github.com/kyverno/kyverno/pkg/utils/slices"
	"github.com/stretchr/testify/assert"
)

func TestFilter(t *testing.T) {
	filtered := utilslices.Filter([]bool{true, false, true}, func(v bool) bool { return v })

	assert.Len(t, filtered, 2)
	assert.False(t, slices.Contains(filtered, false))
}
