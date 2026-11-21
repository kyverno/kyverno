package store

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
