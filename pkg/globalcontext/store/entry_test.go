package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/version"
)

func compileProjection(t *testing.T, expr string) Projection {
	t.Helper()
	prog, errs := compiler.CompileGlobalContextProjection(field.NewPath("expression"), expr)
	require.Empty(t, errs, "expression %q should compile: %v", expr, errs)
	return Projection{Name: "p", Program: prog}
}

func TestProjection_Evaluate_CELNumericTypes(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		input any
		want  any
	}{
		{"int", "object.size()", []any{map[string]any{"a": 1}}, int64(1)},
		{"double", "1.5 * 2.0", nil, float64(3)},
		{"uint", "uint(3)", nil, uint64(3)},
		{"bool", "object.size() > 0", []any{1}, true},
		{"string", "'hello'", nil, "hello"},
		{"null", "null", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := compileProjection(t, tt.expr)
			result, err := proj.Evaluate(context.Background(), tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
			if tt.want != nil {
				assert.IsType(t, tt.want, result)
			}
		})
	}
}

func TestProjection_Evaluate_ListOfMaps(t *testing.T) {
	proj := compileProjection(t, "object.items.filter(i, i.name == 'x').map(i, i)")
	input := map[string]any{
		"items": []any{
			map[string]any{"name": "x", "value": int64(1)},
			map[string]any{"name": "y", "value": int64(2)},
		},
	}
	result, err := proj.Evaluate(context.Background(), input)
	require.NoError(t, err)
	list, ok := result.([]any)
	require.True(t, ok, "result should be []any, got %T", result)
	require.Len(t, list, 1)
	m, ok := list[0].(map[string]any)
	require.True(t, ok, "element should be map[string]any, got %T", list[0])
	assert.Equal(t, "x", m["name"])
	assert.Equal(t, int64(1), m["value"])
}

func TestProjection_Evaluate_NestedOptional(t *testing.T) {
	proj := compileProjection(t, "object.?a.orValue(0)")
	result, err := proj.Evaluate(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), result)

	proj2 := compileProjection(t, "object.?a")
	result2, err := proj2.Evaluate(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.Nil(t, result2)
}

func TestProjection_Evaluate_NonStringMapKeyErrors(t *testing.T) {
	proj := compileProjection(t, "{1: 'a'}")
	_, err := proj.Evaluate(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only string keys are supported")
}

func TestProjection_Evaluate_CELErrorSurfacesAsError(t *testing.T) {
	proj := compileProjection(t, "object.missingField")
	_, err := proj.Evaluate(context.Background(), map[string]any{})
	assert.Error(t, err)
}

// TestProjection_Evaluate_NumericType_CELvsJMESPath pins the design's deliberate choice not
// to normalize CEL numerics: over the identical input, JMESPath's "length(@)" and CEL's
// "object.size()" both count 3 items but surface as different Go types (float64 vs int64).
// This is intentional (see design.md "Numeric semantics"), not a bug.
func TestProjection_Evaluate_NumericType_CELvsJMESPath(t *testing.T) {
	input := []any{1, 2, 3}

	jp := jmespath.New(config.NewDefaultConfiguration(false))
	jpQuery, err := jp.Query("length(@)")
	require.NoError(t, err)
	jmesProj := Projection{Name: "p", JP: jpQuery}
	jmesResult, err := jmesProj.Evaluate(context.Background(), input)
	require.NoError(t, err)

	celProj := compileProjection(t, "object.size()")
	celResult, err := celProj.Evaluate(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, float64(3), jmesResult)
	assert.IsType(t, float64(0), jmesResult, "JMESPath length(@) yields float64")

	assert.Equal(t, int64(3), celResult)
	assert.IsType(t, int64(0), celResult, "CEL object.size() yields int64")

	assert.NotEqual(t, jmesResult, celResult, "the two results must NOT compare equal by ==: they differ in Go type (float64 vs int64) even though both represent the count 3")
}

func TestProjection_Evaluate_JMESPathUnchanged(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	query, err := jp.Query("items | length(@)")
	require.NoError(t, err)

	proj := Projection{Name: "p", JP: query}
	result, err := proj.Evaluate(context.Background(), map[string]any{
		"items": []any{1, 2, 3},
	})
	require.NoError(t, err)
	assert.Equal(t, float64(3), result)
	assert.IsType(t, float64(0), result, "JMESPath numeric results remain float64")
}

type testGCEEntry struct {
	val any
}

func (e *testGCEEntry) Get(projection string) (any, error) {
	return e.val, nil
}

func (e *testGCEEntry) Stop() {}

type testGCEContextProvider struct {
	gStore Store
}

func (p *testGCEContextProvider) GetGlobalReference(name, projection string) (any, error) {
	entry, ok := p.gStore.Get(name)
	if !ok {
		return nil, fmt.Errorf("entry %q not found", name)
	}
	return entry.Get(projection)
}

func TestProjection_Evaluate_CELKubernetesTypes(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"quantity", "quantity('2Gi')"},
		{"ip", "ip('10.1.2.3')"},
		{"cidr", "cidr('10.0.0.0/8')"},
		{"url", "url('https://example.com/path')"},
		{"semver", "semver('1.2.3')"},
	}

	env, err := cel.NewEnv(compiler.DefaultEnvOptions()...)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := compileProjection(t, tt.expr)
			result, err := proj.Evaluate(context.Background(), nil)
			require.NoError(t, err)

			// Result should remain a ref.Val rather than unwrapped to a raw Go struct
			refVal, ok := result.(ref.Val)
			require.True(t, ok, "result for %s should be a ref.Val, got %T", tt.name, result)
			assert.False(t, types.IsError(refVal))

			// cel-go's NativeToValue must accept the result without "unsupported conversion to ref.Val" error
			converted := env.TypeAdapter().NativeToValue(result)
			assert.False(t, types.IsError(converted), "NativeToValue failed for %s: %v", tt.name, converted)
		})
	}
}

func TestProjection_Evaluate_CELKubernetesTypes_PolicyRead(t *testing.T) {
	tests := []struct {
		name       string
		projection string
		policyExpr string
		want       any
	}{
		{
			name:       "quantity comparison",
			projection: "quantity('2Gi')",
			policyExpr: `globalContext.get("limits", "").isGreaterThan(quantity("1Gi"))`,
			want:       true,
		},
		{
			name:       "ip check",
			projection: "ip('10.1.2.3')",
			policyExpr: `globalContext.get("limits", "").family() == 4`,
			want:       true,
		},
		{
			name:       "cidr contains IP",
			projection: "cidr('10.0.0.0/8')",
			policyExpr: `globalContext.get("limits", "").containsIP(ip("10.1.2.3"))`,
			want:       true,
		},
		{
			name:       "url scheme",
			projection: "url('https://example.com/path')",
			policyExpr: `globalContext.get("limits", "").getScheme() == "https"`,
			want:       true,
		},
		{
			name:       "semver comparison",
			projection: "semver('1.2.3')",
			policyExpr: `globalContext.get("limits", "").isGreaterThan(semver("1.0.0"))`,
			want:       true,
		},
		{
			name:       "quantity inside map",
			projection: "{'maxMemory': quantity('2Gi')}",
			policyExpr: `globalContext.get("limits", "")['maxMemory'].isGreaterThan(quantity("1Gi"))`,
			want:       true,
		},
		{
			name:       "quantity inside list",
			projection: "[quantity('500Mi'), quantity('2Gi')]",
			policyExpr: `globalContext.get("limits", "")[1].isGreaterThan(quantity("1Gi"))`,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proj := compileProjection(t, tt.projection)
			result, err := proj.Evaluate(context.Background(), nil)
			require.NoError(t, err)

			gStore := New(0)
			require.NoError(t, gStore.Set("limits", &testGCEEntry{val: result}))

			cp := &testGCEContextProvider{gStore: gStore}
			env, err := cel.NewEnv(
				append(
					compiler.DefaultEnvOptions(),
					globalcontext.Lib(
						globalcontext.Context{ContextInterface: cp},
						version.MajorMinor(1, 18),
					),
				)...,
			)
			require.NoError(t, err)

			ast, iss := env.Compile(tt.policyExpr)
			require.NoError(t, iss.Err())

			prg, err := compiler.NewProgram(env, ast)
			require.NoError(t, err)

			out, _, err := prg.Eval(map[string]any{
				"globalContext": globalcontext.Context{ContextInterface: cp},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, out.Value())
		})
	}
}
