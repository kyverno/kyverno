package context

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ctx struct {
	GetConfigMapFunc func(string, string) (unstructured.Unstructured, error)
}

func (mock *ctx) GetConfigMap(ns string, n string) (unstructured.Unstructured, error) {
	return mock.GetConfigMapFunc(ns, n)
}

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
		"context": Context{&ctx{
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
