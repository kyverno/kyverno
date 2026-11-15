package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestVariablesProviderRegisterFieldDeduplicatesNames(t *testing.T) {
	env, err := NewBaseEnv()
	require.NoError(t, err)

	provider := NewVariablesProvider(env.CELTypeProvider())
	typeName := VariablesType.DeclaredTypeName()

	names, found := provider.FindStructFieldNames(typeName)
	require.True(t, found)
	require.Empty(t, names)

	provider.RegisterField("count", cel.IntType)
	provider.RegisterField("label", cel.StringType)
	provider.RegisterField("count", cel.IntType)    // same type
	provider.RegisterField("count", cel.StringType) // updated type
	provider.RegisterField("active", cel.BoolType)
	provider.RegisterField("label", cel.StringType)

	names, found = provider.FindStructFieldNames(typeName)
	require.True(t, found)
	require.Equal(t, []string{"count", "label", "active"}, names)

	for _, tc := range []struct {
		name string
		typ  *types.Type
	}{
		{"count", cel.StringType},
		{"label", cel.StringType},
		{"active", cel.BoolType},
	} {
		fieldType, found := provider.FindStructFieldType(typeName, tc.name)
		require.True(t, found, "field %q", tc.name)
		require.True(t, fieldType.Type.IsExactType(tc.typ), "field %q", tc.name)
	}

	_, found = provider.FindStructFieldType(typeName, "missing")
	require.False(t, found)
}

func TestCompileVariablesDeduplicatesFieldNames(t *testing.T) {
	for _, tc := range []struct {
		name  string
		trace bool
	}{
		{"untraced", false},
		{"traced", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, err := NewBaseEnv()
			require.NoError(t, err)

			provider := NewVariablesProvider(env.CELTypeProvider())
			env, err = env.Extend(
				cel.Variable(VariablesKey, VariablesType),
				cel.CustomTypeProvider(provider),
			)
			require.NoError(t, err)

			variables := []admissionregistrationv1.Variable{
				{Name: "count", Expression: "1"},
				{Name: "label", Expression: `"first"`},
				{Name: "count", Expression: `"updated"`},
			}
			programs, traced, errs := CompileVariablesWithTrace(nil, env, provider, tc.trace, variables...)
			require.Empty(t, errs)
			require.Len(t, programs, 2)

			names, found := provider.FindStructFieldNames(VariablesType.DeclaredTypeName())
			require.True(t, found)
			require.Equal(t, []string{"count", "label"}, names)

			fieldType, found := provider.FindStructFieldType(VariablesType.DeclaredTypeName(), "count")
			require.True(t, found)
			require.True(t, fieldType.Type.IsExactType(cel.StringType))

			output, _, err := programs["count"].Eval(map[string]any{})
			require.NoError(t, err)
			require.Equal(t, "updated", output.Value())

			if tc.trace {
				require.Len(t, traced, 2)
				require.NotNil(t, traced["count"].Traced)
			} else {
				require.Nil(t, traced)
			}
		})
	}
}
