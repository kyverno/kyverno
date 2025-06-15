package globalcontext

import (
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_impl_get_string(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("globalContext", ContextType),
		Lib(),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`globalContext.Get("foo")`)
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
		expectedValue: structpb.NullValue(0),
	}, {
		name: "global context entry returns error",
		gctxStoreData: map[string]store.Entry{
			"foo": &MockEntry{Err: errors.New("get entry error")},
		},
		expectedError: "get entry error",
	}, {
		name: "global context entry returns string",
		gctxStoreData: map[string]store.Entry{
			"foo": &MockEntry{Data: "stringValue"},
		},
		expectedValue: "stringValue",
	}, {
		name: "global context entry returns map",
		gctxStoreData: map[string]store.Entry{
			"foo": &MockEntry{Data: map[string]any{"key": "value"}},
		},
		expectedValue: map[string]any{"key": "value"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockGctxStore{Data: tt.gctxStoreData}
			data := map[string]any{
				"globalContext": Context{&ContextMock{
					GetGlobalReferenceFunc: func(name string, path string) (any, error) {
						ent, ok := mockStore.Get(name)
						if !ok {
							return nil, nil
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

func Test_impl_get_string_string(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("globalContext", ContextType),
		Lib(),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`globalContext.Get("foo", "bar")`)
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
		expectedValue: structpb.NullValue(0),
	}, {
		name: "global context entry returns error",
		gctxStoreData: map[string]store.Entry{
			"foo": &MockEntry{Err: errors.New("get entry error")},
		},
		expectedError: "get entry error",
	}, {
		name: "global context entry returns string",
		gctxStoreData: map[string]store.Entry{
			"foo": &MockEntry{Data: "stringValue"},
		},
		expectedValue: "stringValue",
	}, {
		name: "global context entry returns map",
		gctxStoreData: map[string]store.Entry{
			"foo": &MockEntry{Data: map[string]any{"key": "value"}},
		},
		expectedValue: map[string]any{"key": "value"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockGctxStore{Data: tt.gctxStoreData}
			data := map[string]any{
				"globalContext": Context{&ContextMock{
					GetGlobalReferenceFunc: func(name string, path string) (any, error) {
						ent, ok := mockStore.Get(name)
						if !ok {
							return nil, nil
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

func Test_impl_get_string_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("globalContext", ContextType),
		Lib(),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("foo")},
		want: types.NewErr("unsupported native conversion from string to 'globalcontext.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false)},
		want: types.NewErr("type conversion error from bool to 'string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.get_string(tt.args[0], tt.args[1])
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_impl_get_string_string_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("globalContext", ContextType),
		Lib(),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
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
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false), types.String("foo")},
		want: types.NewErr("type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 3",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("foo"), types.Bool(false)},
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
