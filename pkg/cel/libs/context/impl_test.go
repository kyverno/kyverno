package context

import (
	"context"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_impl_get_configmap_string_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("context", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`context.GetConfigMap("foo","bar")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	called := false
	data := map[string]any{
		"context": Context{&MockCtx{
			GetConfigMapFunc: func(string, string) (unstructured.Unstructured, error) {
				called = true
				return unstructured.Unstructured{}, nil
			},
		}},
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.True(t, called)
}

func Test_impl_get_globalreference_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("context", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`context.GetGlobalReference("foo")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	called := false
	data := map[string]any{
		"context": Context{&MockCtx{
			GetGlobalReferenceFunc: func(string) (any, error) {
				type foo struct {
					s string
				}
				called = true
				return foo{"bar"}, nil
			},
		}},
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.True(t, called)
}

func Test_impl_get_imagedata_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("context", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`context.GetImageData("ghcr.io/kyverno/kyverno:latest")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"context": Context{&MockCtx{
			GetImageDataFunc: func(image string) (*imagedataloader.ImageData, error) {
				idl, err := imagedataloader.New(nil)
				assert.NoError(t, err)
				return idl.FetchImageData(context.TODO(), image)
			},
		}},
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	img := out.Value().(*imagedataloader.ImageData)
	assert.Equal(t, img.Tag, "latest")
	assert.True(t, strings.HasPrefix(img.ResolvedImage, "ghcr.io/kyverno/kyverno:latest@sha256:"))
	assert.True(t, img.ConfigData != nil)
	assert.True(t, img.Manifest != nil)
	assert.True(t, img.ImageIndex != nil)
}

func Test_impl_get_resource_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("context", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`context.GetResource("apps/v1", "Deployment", "default", "nginx")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"context": Context{&MockCtx{
			GetResourcesFunc: func(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
				return &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": apiVersion,
						"kind":       resource,
						"metadata": map[string]any{
							"name":      name,
							"namespace": namespace,
						},
					},
				}, nil
			},
		}},
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	object := out.Value().(map[string]any)
	assert.Equal(t, object["apiVersion"].(string), "apps/v1")
	assert.Equal(t, object["kind"].(string), "Deployment")
}

func Test_impl_list_resource_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("context", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`context.ListResource("apps/v1", "Deployment", "default")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"context": Context{&MockCtx{
			ListResourcesFunc: func(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
				return &unstructured.UnstructuredList{
					Items: []unstructured.Unstructured{
						{
							Object: map[string]any{
								"apiVersion": apiVersion,
								"kind":       resource,
								"metadata": map[string]any{
									"name":      "nginx",
									"namespace": namespace,
								},
							},
						},
					},
				}, nil
			},
		}},
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	object := out.Value().(map[string]any)
	assert.Equal(t, object["items"].([]any)[0].(map[string]any)["apiVersion"].(string), "apps/v1")
	assert.Equal(t, object["items"].([]any)[0].(map[string]any)["kind"].(string), "Deployment")
}
