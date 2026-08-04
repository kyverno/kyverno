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

func TestTwoVarComprehensions_TransformList(t *testing.T) {
	env, err := NewBaseEnv()
	require.NoError(t, err)

	ast, issues := env.Compile(`[10, 20, 30].transformList(i, v, v + i)`)
	if issues != nil {
		require.NoError(t, issues.Err())
	}
	prg, err := env.Program(ast)
	require.NoError(t, err)

	out, _, err := prg.Eval(map[string]any{})
	require.NoError(t, err)

	list, ok := out.(traits.Lister)
	require.True(t, ok)
	assert.Equal(t, 3, int(list.Size().(types.Int)))
	assert.Equal(t, int64(10), list.Get(types.Int(0)).Value())
	assert.Equal(t, int64(21), list.Get(types.Int(1)).Value())
	assert.Equal(t, int64(32), list.Get(types.Int(2)).Value())
}

func TestTwoVarComprehensions_TransformMap(t *testing.T) {
	env, err := cel.NewEnv(DynamicResourceEnvOptions()...)
	require.NoError(t, err)

	ast, issues := env.Compile(`{"a": 1, "b": 2}.transformMap(k, v, v * 10)`)
	if issues != nil {
		require.NoError(t, issues.Err())
	}
	prg, err := env.Program(ast)
	require.NoError(t, err)

	out, _, err := prg.Eval(map[string]any{})
	require.NoError(t, err)

	m, ok := out.(traits.Mapper)
	require.True(t, ok)
	aVal := m.Get(types.String("a"))
	bVal := m.Get(types.String("b"))
	assert.Equal(t, int64(10), aVal.Value())
	assert.Equal(t, int64(20), bVal.Value())
}

func TestTwoVarComprehensions_TransformMapEntry(t *testing.T) {
	env, err := cel.NewEnv(DynamicResourceEnvOptions()...)
	require.NoError(t, err)

	ast, issues := env.Compile(`{"greeting": "hello"}.transformMapEntry(k, v, {v: k})`)
	if issues != nil {
		require.NoError(t, issues.Err())
	}
	prg, err := env.Program(ast)
	require.NoError(t, err)

	out, _, err := prg.Eval(map[string]any{})
	require.NoError(t, err)

	m, ok := out.(traits.Mapper)
	require.True(t, ok)
	val := m.Get(types.String("hello"))
	assert.Equal(t, "greeting", val.Value())
}

func TestTwoVarComprehensions_AllWithTwoVars(t *testing.T) {
	env, err := NewBaseEnv()
	require.NoError(t, err)

	ast, issues := env.Compile(`[10, 20, 30].all(i, v, v > i)`)
	if issues != nil {
		require.NoError(t, issues.Err())
	}
	prg, err := env.Program(ast)
	require.NoError(t, err)

	out, _, err := prg.Eval(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, true, out.Value())
}

func TestTwoVarComprehensions_ExistsWithTwoVars(t *testing.T) {
	env, err := cel.NewEnv(DynamicResourceEnvOptions()...)
	require.NoError(t, err)

	ast, issues := env.Compile(`{"x": 1, "y": 2}.exists(k, v, k == "x" && v == 1)`)
	if issues != nil {
		require.NoError(t, issues.Err())
	}
	prg, err := env.Program(ast)
	require.NoError(t, err)

	out, _, err := prg.Eval(map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, true, out.Value())
}
