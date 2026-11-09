package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesProviderRegisterFieldDeduplicatesNames(t *testing.T) {
	t.Parallel()
	env, err := NewBaseEnv()
	require.NoError(t, err)

	provider := NewVariablesProvider(env.CELTypeProvider())
	provider.RegisterField("a", cel.BoolType)
	provider.RegisterField("count", cel.IntType)
	provider.RegisterField("count", cel.StringType)
	provider.RegisterField("a", cel.BoolType)

	names, found := provider.FindStructFieldNames(VariablesType.DeclaredTypeName())
	require.True(t, found)
	assert.Equal(t, []string{"a", "count"}, names)

	fieldType, found := provider.FindStructFieldType(VariablesType.DeclaredTypeName(), "count")
	require.True(t, found)
	assert.True(t, fieldType.Type.IsExactType(cel.StringType))

	_, found = provider.FindStructFieldType(VariablesType.DeclaredTypeName(), "nonexistent")
	assert.False(t, found)
}
