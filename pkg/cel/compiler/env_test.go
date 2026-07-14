package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/traits"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/version"
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

	expr := `[1, "two"]`

	_, issues := defaultEnv.Compile(expr)
	require.NotNil(t, issues)
	require.Error(t, issues.Err())

	_, issues = dynamicEnv.Compile(expr)
	if issues != nil {
		require.NoError(t, issues.Err())
	}
}

func TestDynamicResourceEnvOptionsWithCompat_OrValueOnConcrete(t *testing.T) {
	dynamicCompatEnv, err := cel.NewEnv(DynamicResourceEnvOptionsWithCompat()...)
	require.NoError(t, err)

	ast, issues := dynamicCompatEnv.Compile(`[1,2].orValue([])`)
	if issues != nil {
		require.NoError(t, issues.Err())
	}

	program, err := dynamicCompatEnv.Program(ast)
	require.NoError(t, err)

	out, _, err := program.Eval(map[string]any{})
	require.NoError(t, err)

	list, ok := out.(traits.Lister)
	require.True(t, ok)
	assert.Equal(t, 2, int(list.Size().(types.Int)))
}

func TestEnvOptionsForVersion(t *testing.T) {
	legacyOpts := VersionedEnvOptions{
		IntroducedVersion: version.MajorMinor(1, 0),
		RemovedVersion:    version.MajorMinor(2, 0),
		EnvOptions:        []cel.EnvOption{cel.HomogeneousAggregateLiterals()},
	}
	currentOpts := VersionedEnvOptions{
		IntroducedVersion: version.MajorMinor(2, 0),
		EnvOptions:        []cel.EnvOption{},
	}

	v1Env, err := cel.NewEnv(EnvOptionsForVersion(version.MajorMinor(1, 5), legacyOpts, currentOpts)...)
	require.NoError(t, err)
	_, issues := v1Env.Compile(`[1, "two"]`)
	require.NotNil(t, issues)
	require.Error(t, issues.Err())

	v2Env, err := cel.NewEnv(EnvOptionsForVersion(version.MajorMinor(2, 0), legacyOpts, currentOpts)...)
	require.NoError(t, err)
	_, issues = v2Env.Compile(`[1, "two"]`)
	if issues != nil {
		require.NoError(t, issues.Err())
	}
}
