package generator

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

func Test_apply_generator_string_list(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("generator", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`
generator.Apply(
	"default",
	[
		{
			"apiVersion": dyn("apps/v1"),
			"kind":       dyn("Deployment"),
			"metadata": dyn({
				"name":      "name",
				"namespace": "namespace",
			}),
		},
	]
)`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"generator": Context{&ContextMock{
			GenerateResourcesFunc: func(namespace string, dataList []map[string]any) error {
				assert.Equal(t, "default", namespace)
				assert.Len(t, dataList, 1)
				assert.Equal(t, dataList[0]["apiVersion"].(string), "apps/v1")
				assert.Equal(t, dataList[0]["kind"].(string), "Deployment")
				assert.Equal(t, dataList[0]["metadata"].(map[string]any)["name"], "name")
				assert.Equal(t, dataList[0]["metadata"].(map[string]any)["namespace"], "namespace")
				return nil
			},
		},
		}}
	_, _, err = prog.Eval(data)
	assert.NoError(t, err)
}

func Test_apply_generator_string_list_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("generator", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("default"), types.NewListType(types.NewMapType(types.StringType, types.AnyType))},
		want: types.NewErr("invalid arg 0: unsupported native conversion from string to 'generator.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false), types.NewListType(types.NewMapType(types.StringType, types.AnyType))},
		want: types.NewErr("invalid arg 1: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 3",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("default"), types.Bool(false), types.String("ns"), types.NewMapType(types.StringType, types.AnyType)},
		want: types.NewErr("invalid arg 2: type conversion error from bool to '[]*structpb.Struct'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.apply_generator_string_list(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}
