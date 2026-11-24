package processor

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type named string

func (n named) GetName() string { return string(n) }

func overrideVars(t *testing.T) *variables.Variables {
	t.Helper()
	tier := func(v string) map[string]interface{} {
		return map[string]interface{}{"request.oldObject": map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"tier": v}}}}
	}
	vars, err := variables.New(nil, nil, "", "", &v1alpha1.ValuesSpec{Policies: []v1alpha1.Policy{
		{Name: "pol-a", Resources: []v1alpha1.Resource{{Name: "p1", Values: tier("old")}}},
		{Name: "pol-b", Resources: []v1alpha1.Resource{{Name: "p1", Values: tier("old")}}},
		{Name: "pol-c", Resources: []v1alpha1.Resource{{Name: "p1", Values: tier("other")}}},
	}})
	require.NoError(t, err)
	return vars
}

func TestGroupByOldObjectOverride(t *testing.T) {
	groups, err := GroupByOldObjectOverride(overrideVars(t), []string{"pol-a", "pol-none", "pol-b", "pol-c"}, "p1", "UPDATE")
	require.NoError(t, err)
	var names [][]string
	for _, g := range groups {
		names = append(names, g.Names)
	}
	// Policies with equal overrides share a group; a policy without one gets its own, with a nil override.
	assert.Equal(t, [][]string{{"pol-a", "pol-b"}, {"pol-none"}, {"pol-c"}}, names)
	assert.Nil(t, groups[1].Override)

	// CREATE has no old object: one group, no override, and nothing is resolved.
	groups, err = GroupByOldObjectOverride(overrideVars(t), []string{"pol-a", "pol-c"}, "p1", "CREATE")
	require.NoError(t, err)
	assert.Equal(t, []OverrideGroup{{Names: []string{"pol-a", "pol-c"}}}, groups)
}

func TestMutatingOldObjectOverride(t *testing.T) {
	vars := overrideVars(t)
	got, err := mutatingOldObjectOverride(vars, []named{"pol-a", "pol-b"}, "p1", "UPDATE")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"tier": "old"}}}, got)

	_, err = mutatingOldObjectOverride(vars, []named{"pol-a", "pol-c"}, "p1", "UPDATE")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pol-a")
	assert.Contains(t, err.Error(), "pol-c")

	// A policy that declares no override (one matching other kinds, say) isn't a conflict.
	got, err = mutatingOldObjectOverride(vars, []named{"pol-none", "pol-a"}, "p1", "UPDATE")
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"tier": "old"}}}, got)

	// CREATE has no old object, so conflicting overrides don't matter.
	got, err = mutatingOldObjectOverride(vars, []named{"pol-a", "pol-c"}, "p1", "CREATE")
	require.NoError(t, err)
	assert.Nil(t, got)
}
