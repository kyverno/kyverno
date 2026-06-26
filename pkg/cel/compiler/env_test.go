package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestEnvOptions_HomogeneousAggregateBehavior(t *testing.T) {
	defaultEnv, err := cel.NewEnv(DefaultEnvOptions()...)
	require.NoError(t, err)

	dynamicEnv, err := cel.NewEnv(DynamicResourceEnvOptions()...)
	require.NoError(t, err)

	// heterogeneous aggregate literal
	expr := `[1, "two"]`

	_, issues := defaultEnv.Compile(expr)
	require.NotNil(t, issues)
	require.Error(t, issues.Err())

	_, issues = dynamicEnv.Compile(expr)
	require.Nil(t, issues)
}
