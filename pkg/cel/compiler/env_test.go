package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/require"
)

func TestEnvOptions_HomogeneousAggregateBehavior(t *testing.T) {
	defaultEnv, err := cel.NewEnv(DefaultEnvOptions()...)
	require.NoError(t, err)

	dynamicEnv, err := cel.NewEnv(DynamicResourceEnvOptions()...)
	require.NoError(t, err)

	// heterogeneous aggregate literal
	expr := `[1, "two"]`

	_, issues := defaultEnv.Compile(expr)
	require.Error(t, issues.Err())

	_, issues = dynamicEnv.Compile(expr)
	require.NoError(t, issues.Err())
}
