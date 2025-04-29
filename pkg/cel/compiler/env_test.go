package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEnv(t *testing.T) {
	got, err := NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, got)
}
