package resource

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_impl_get_resource_string_string_string_string(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("resource", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`resource.Get("apps/v1", "deployments", "default", "nginx")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"resource": Context{&ContextMock{
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

func Test_impl_get_resource_string_string_string_string_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("resource", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "not enough args",
		args: nil,
		want: types.NewErr("expected 5 arguments, got %d", 0),
	}, {
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("v1"), types.String("pods"), types.String("ns"), types.String("name")},
		want: types.NewErr("invalid arg 0: unsupported native conversion from string to 'resource.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false), types.String("pods"), types.String("ns"), types.String("name")},
		want: types.NewErr("invalid arg 1: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 3",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("v1"), types.Bool(false), types.String("ns"), types.String("name")},
		want: types.NewErr("invalid arg 2: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 4",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("v1"), types.String("pods"), types.Bool(false), types.String("name")},
		want: types.NewErr("invalid arg 3: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 5",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("v1"), types.String("pods"), types.String("ns"), types.Bool(false)},
		want: types.NewErr("invalid arg 4: type conversion error from bool to 'string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.get_resource_string_string_string_string(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_impl_list_resources_string_string_string(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("resource", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`resource.List("apps/v1", "deployments", "default")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"resource": Context{&ContextMock{
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

func Test_impl_list_resources_string_string_string_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("resource", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "not enough args",
		args: nil,
		want: types.NewErr("expected 4 arguments, got %d", 0),
	}, {
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("v1"), types.String("pods"), types.String("name")},
		want: types.NewErr("invalid arg 0: unsupported native conversion from string to 'resource.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false), types.String("pods"), types.String("name")},
		want: types.NewErr("invalid arg 1: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 3",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("v1"), types.Bool(false), types.String("name")},
		want: types.NewErr("invalid arg 2: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 4",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("v1"), types.String("pods"), types.Bool(false)},
		want: types.NewErr("invalid arg 3: type conversion error from bool to 'string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.list_resources_string_string_string(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_post_resource_string_string_string_map(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("resource", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`
resource.Post(
	"apps/v1",
	"deployments",
	"default",
	{
		"apiVersion": dyn("apps/v1"),
		"kind":       dyn("Deployment"),
		"metadata": dyn({
			"name":      "name",
			"namespace": "namespace",
		}),
	}
)`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"resource": Context{&ContextMock{
			PostResourceFunc: func(apiVersion, resource, namespace string, payload map[string]any) (*unstructured.Unstructured, error) {
				assert.Equal(t, payload["apiVersion"].(string), "apps/v1")
				assert.Equal(t, payload["kind"].(string), "Deployment")
				assert.Equal(t, payload["metadata"].(map[string]any)["name"], "name")
				assert.Equal(t, payload["metadata"].(map[string]any)["namespace"], "namespace")
				return &unstructured.Unstructured{
					Object: payload,
				}, nil
			},
		},
		}}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	object := out.Value().(map[string]any)
	assert.Equal(t, object["apiVersion"].(string), "apps/v1")
	assert.Equal(t, object["kind"].(string), "Deployment")
	assert.Equal(t, object["metadata"].(map[string]any)["name"], "name")
	assert.Equal(t, object["metadata"].(map[string]any)["namespace"], "namespace")
}

func Test_impl_post_resource_string_string_string_map_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("resource", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "not enough args",
		args: nil,
		want: types.NewErr("expected 5 arguments, got %d", 0),
	}, {
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("v1"), types.String("pods"), types.String("ns"), types.NewMapType(types.StringType, types.AnyType)},
		want: types.NewErr("invalid arg 0: unsupported native conversion from string to 'resource.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false), types.String("pods"), types.String("ns"), types.NewMapType(types.StringType, types.AnyType)},
		want: types.NewErr("invalid arg 1: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 3",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("v1"), types.Bool(false), types.String("ns"), types.NewMapType(types.StringType, types.AnyType)},
		want: types.NewErr("invalid arg 2: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 4",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("v1"), types.String("pods"), types.Bool(false), types.NewMapType(types.StringType, types.AnyType)},
		want: types.NewErr("invalid arg 3: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 5",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("v1"), types.String("pods"), types.String("ns"), types.Bool(false)},
		want: types.NewErr("invalid arg 4: type conversion error from bool to '*structpb.Struct'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.post_resource_string_string_string_map(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_impl_post_resource_string_string_map_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("resource", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "not enough args",
		args: nil,
		want: types.NewErr("expected 4 arguments, got %d", 0),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.post_resource_string_string_map(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}
