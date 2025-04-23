package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEnv(t *testing.T) {
	got, err := NewEnv()
	assert.NoError(t, err)
	assert.NotNil(t, got)
}
