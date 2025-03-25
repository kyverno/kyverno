package globalcontext

import (
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_impl_get_string_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("globalcontext", ContextType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`globalcontext.Get("foo", "bar")`)
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
	}{{
		name:          "global context entry not found",
		gctxStoreData: map[string]store.Entry{},
		expectedError: "global context entry not found",
	}, {
		name: "global context entry returns error",
		gctxStoreData: map[string]store.Entry{
			"foo": &resource.MockEntry{Err: errors.New("get entry error")},
		},
		expectedError: "get entry error",
	}, {
		name: "global context entry returns string",
		gctxStoreData: map[string]store.Entry{
			"foo": &resource.MockEntry{Data: "stringValue"},
		},
		expectedValue: "stringValue",
	}, {
		name: "global context entry returns map",
		gctxStoreData: map[string]store.Entry{
			"foo": &resource.MockEntry{Data: map[string]any{"key": "value"}},
		},
		expectedValue: map[string]any{"key": "value"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &resource.MockGctxStore{Data: tt.gctxStoreData}
			data := map[string]any{
				"globalcontext": Context{&resource.MockCtx{
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

func Test_impl_get_string_string_error(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "not enough args",
		args: nil,
		want: types.NewErr("expected 3 arguments, got %d", 0),
	}, {
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("foo"), types.String("foo")},
		want: types.NewErr("unsupported native conversion from string to 'globalcontext.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{base.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false), types.String("foo")},
		want: types.NewErr("type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 3",
		args: []ref.Val{base.CELTypeAdapter().NativeToValue(Context{}), types.String("foo"), types.Bool(false)},
		want: types.NewErr("type conversion error from bool to 'string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.get_string_string(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}
