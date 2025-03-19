package resource

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_impl_get_resource_string_string_string_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("resource", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`resource.Get("apps/v1", "deployments", "default", "nginx")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"resource": Context{&MockCtx{
			GetResourceFunc: func(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
				return &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name":      name,
							"namespace": namespace,
						},
					},
				}, nil
			},
		},
		},
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	object := out.Value().(map[string]any)
	assert.Equal(t, object["apiVersion"].(string), "apps/v1")
	assert.Equal(t, object["kind"].(string), "Deployment")
}

func Test_impl_list_resources_string_string_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("resource", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`resource.List("apps/v1", "deployments", "default")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"resource": Context{&MockCtx{
			ListResourcesFunc: func(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
				return &unstructured.UnstructuredList{
					Items: []unstructured.Unstructured{
						{
							Object: map[string]any{
								"apiVersion": "apps/v1",
								"kind":       "Deployment",
								"metadata": map[string]any{
									"name":      "nginx",
									"namespace": namespace,
								},
							},
						},
					},
				}, nil
			},
		},
		}}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	object := out.Value().(map[string]any)
	assert.Equal(t, object["items"].([]any)[0].(map[string]any)["apiVersion"].(string), "apps/v1")
	assert.Equal(t, object["items"].([]any)[0].(map[string]any)["kind"].(string), "Deployment")
}
