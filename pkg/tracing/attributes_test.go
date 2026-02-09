package tracing

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringValueWithinLimit(t *testing.T) {
	input := "short string"
	got := StringValue(input)
	assert.Equal(t, "short string", got)
}

func TestStringValueAtLimit(t *testing.T) {
	input := strings.Repeat("a", 256)
	got := StringValue(input)
	assert.Equal(t, input, got)
	assert.Len(t, got, 256)
}

func TestStringValueExceedsLimit(t *testing.T) {
	input := strings.Repeat("a", 300)
	got := StringValue(input)

	assert.Len(t, got, 256)
	assert.True(t, strings.HasSuffix(got, "..."))
	assert.Equal(t, strings.Repeat("a", 253)+"...", got)
}

func TestStringValueEmpty(t *testing.T) {
	input := ""
	got := StringValue(input)
	assert.Equal(t, "", got)
}
