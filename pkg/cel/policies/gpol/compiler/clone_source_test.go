package compiler

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/sdk/extensions/cel/libs/generator"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type cloneSourceContext struct {
	libs.Context
	reads  []string
	writes []string
}

func (c *cloneSourceContext) GetResource(_, _, namespace, _ string) (*unstructured.Unstructured, error) {
	c.reads = append(c.reads, namespace)
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "source", "namespace": namespace},
	}}, nil
}

func (c *cloneSourceContext) ListResources(_, _, namespace string, _ map[string]string) (*unstructured.UnstructuredList, error) {
	c.reads = append(c.reads, namespace)
	return &unstructured.UnstructuredList{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMapList"}}, nil
}

func (c *cloneSourceContext) GenerateResources(namespace string, _ []map[string]any) error {
	c.writes = append(c.writes, namespace)
	return nil
}

// Namespaced resource overloads bind reads to the policy namespace and reject
// explicit namespace arguments. Exercise the environment used by the GPOL compiler.
func TestGeneratingPolicyCloneSourceNamespaces(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, namespace, expression string
		compileError                bool
		reads, writes               []string
	}{
		{"namespaced get", "tenant", `generator.Apply([resource.Get('v1', 'configmaps', 'source')])`, false, []string{"tenant"}, []string{"tenant"}},
		{"namespaced list", "tenant", `generator.Apply([resource.List('v1', 'configmaps')])`, false, []string{"tenant"}, []string{"tenant"}},
		{"GVR get rejects namespace", "tenant", `generator.Apply([resource.Get(resource.ToGVR('v1', 'ConfigMap'), 'other', 'source')])`, true, nil, nil},
		{"GVR list rejects namespace", "tenant", `generator.Apply([resource.List(resource.ToGVR('v1', 'ConfigMap'), 'other')])`, true, nil, nil},
		{"get rejects namespace", "tenant", `generator.Apply([resource.Get('v1', 'configmaps', 'other', 'source')])`, true, nil, nil},
		{"list rejects namespace", "tenant", `generator.Apply([resource.List('v1', 'configmaps', 'other')])`, true, nil, nil},
		{"get rejects variable namespace", "tenant", `generator.Apply([resource.Get('v1', 'configmaps', object.metadata.namespace, 'source')])`, true, nil, nil},
		{"cluster get", "", `generator.Apply('tenant', [resource.Get('v1', 'configmaps', 'other', 'source')])`, false, []string{"other"}, []string{"tenant"}},
		{"cluster list", "", `generator.Apply('tenant', [resource.List('v1', 'configmaps', 'other')])`, false, []string{"other"}, []string{"tenant"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := &cloneSourceContext{Context: libs.NewFakeContextProvider()}
			env, _, err := (&compilerImpl{}).createBaseGpolEnv(provider, tt.namespace)
			require.NoError(t, err)
			ast, issues := env.Compile(tt.expression)
			if tt.compileError {
				require.Error(t, issues.Err())
			} else {
				require.NoError(t, issues.Err())
				program, err := env.Program(ast)
				require.NoError(t, err)
				_, _, err = program.Eval(map[string]any{
					"resource":  resource.Context{ContextInterface: provider},
					"generator": generator.Context{ContextInterface: provider},
				})
				require.NoError(t, err)
			}
			require.Equal(t, tt.reads, provider.reads)
			require.Equal(t, tt.writes, provider.writes)
		})
	}
}
