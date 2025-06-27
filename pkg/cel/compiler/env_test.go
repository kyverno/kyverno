package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBaseEnv(t *testing.T) {
	got, err := NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, got)
}

func TestNewMatchImageEnv(t *testing.T) {
	got, err := NewMatchImageEnv()
	assert.NoError(t, err)
	assert.NotNil(t, got)
}
