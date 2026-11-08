package compiler

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/require"
)

func TestMayReadSecretWithVariablesTracksAliases(t *testing.T) {
	env, err := cel.NewEnv(
		cel.Variable("variables", cel.DynType),
		cel.Variable("object", cel.DynType),
	)
	require.NoError(t, err)

	for _, expression := range []string{
		"variables.secretValue",
		"[variables.secretValue].map(v, v)[0].dbPass",
		"[variables].map(v, v.secretValue)[0]",
	} {
		t.Run(expression, func(t *testing.T) {
			ast, issues := env.Compile(expression)
			require.NoError(t, issues.Err())
			require.True(t, MayReadSecretWithVariables(ast, map[string]bool{"secretValue": true}))
		})
	}

	ast, issues := env.Compile("object.metadata.name")
	require.NoError(t, issues.Err())
	require.False(t, MayReadSecretWithVariables(ast, map[string]bool{"secretValue": true}))

	ast, issues = env.Compile("variables.publicName")
	require.NoError(t, issues.Err())
	require.False(t, MayReadSecretWithVariables(ast, map[string]bool{"secretValue": true}))
}
