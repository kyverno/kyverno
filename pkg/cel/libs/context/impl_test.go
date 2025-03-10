package context

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
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
		},
		}}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.True(t, called)
}

// type mockGctxStore struct {
// 	data map[string]store.Entry
// }

// func (m *mockGctxStore) Get(name string) (store.Entry, bool) {
// 	entry, ok := m.data[name]
// 	return entry, ok
// }

// func (m *mockGctxStore) Set(name string, data store.Entry) {
// 	if m.data == nil {
// 		m.data = make(map[string]store.Entry)
// 	}
// 	m.data[name] = data
// }

// type mockEntry struct {
// 	data any
// 	err  error
// }

// func (m *mockEntry) Get(_ string) (any, error) {
// 	return m.data, m.err
// }

// func (m *mockEntry) Stop() {}

func Test_impl_get_globalreference_string_string(t *testing.T) {
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
	ast, issues := env.Compile(`context.GetGlobalReference("foo", "bar")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)

	tests := []struct {
		name          string
		gctxStoreData map[string]store.Entry
		expectedValue any
		expectedError string
	}{
		{
			name:          "global context entry not found",
			gctxStoreData: map[string]store.Entry{},
			expectedError: "global context entry not found",
		},
		{
			name: "global context entry returns error",
			gctxStoreData: map[string]store.Entry{
				"foo": &mockEntry{err: errors.New("get entry error")},
			},
			expectedError: "get entry error",
		},
		{
			name: "global context entry returns string",
			gctxStoreData: map[string]store.Entry{
				"foo": &mockEntry{data: "stringValue"},
			},
			expectedValue: "stringValue",
		},
		{
			name: "global context entry returns map",
			gctxStoreData: map[string]store.Entry{
				"foo": &mockEntry{data: map[string]interface{}{"key": "value"}},
			},
			expectedValue: map[string]interface{}{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockGctxStore{data: tt.gctxStoreData}
			data := map[string]any{
				"context": Context{&MockCtx{
					GetGlobalReferenceFunc: func(name string, path string) (any, error) {
						ent, ok := mockStore.Get(name)
						if !ok {
							return nil, errors.New("global context entry not found")
						}
						return ent.Get(path)
					},
				}},
			}
			out, _, err := prog.Eval(data)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				if tt.expectedValue == nil {
					assert.Nil(t, out.Value())
				} else {
					assert.NotNil(t, out)
					if expectedUnstructured, ok := tt.expectedValue.(unstructured.Unstructured); ok {
						actualUnstructured, ok := out.Value().(unstructured.Unstructured)
						assert.True(t, ok, "Expected unstructured.Unstructured, got %T", out.Value())
						assert.Equal(t, expectedUnstructured, actualUnstructured)
					} else {
						assert.Equal(t, tt.expectedValue, out.Value())
					}
				}
			}
		})
	}
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
		},
		}}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	img := out.Value().(*imagedataloader.ImageData)
	assert.Equal(t, img.Tag, "latest")
	assert.True(t, strings.HasPrefix(img.ResolvedImage, "ghcr.io/kyverno/kyverno:latest@sha256:"))
	assert.True(t, img.ConfigData != nil)
	assert.True(t, img.Manifest != nil)
	assert.True(t, img.ImageIndex != nil)
}

func Test_impl_parse_image_ref_string(t *testing.T) {
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
	ast, issues := env.Compile(`context.ParseImageReference("ghcr.io/kyverno/kyverno:latest")`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"context": Context{&MockCtx{
			ParseImageReferenceFunc: func(image string) (imagedataloader.ImageReference, error) {
				idl, err := imagedataloader.New(nil)
				assert.NoError(t, err)
				return idl.ParseImageReference(image)
			},
		},
		},
	}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	img := out.Value().(imagedataloader.ImageReference)
	assert.Equal(t, img.Tag, "latest")
	assert.Equal(t, img.Identifier, "latest")
	assert.Equal(t, img.Image, "ghcr.io/kyverno/kyverno:latest")
}

func Test_impl_get_resource_string_string_string_string(t *testing.T) {
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
	ast, issues := env.Compile(`context.GetResource("apps/v1", "deployments", "default", "nginx")`)
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
		cel.Variable("context", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`context.ListResources("apps/v1", "deployments", "default")`)
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
