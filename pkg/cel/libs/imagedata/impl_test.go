package imagedata

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/stretchr/testify/assert"
)

func Test_impl_get_imagedata_string(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("image", ContextType),
		Lib(),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`image.GetMetadata("ghcr.io/kyverno/kyverno:latest").resolvedImage`)
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	data := map[string]any{
		"image": Context{&ContextMock{
			GetImageDataFunc: func(image string) (map[string]any, error) {
				idl, err := imagedataloader.New(nil)
				assert.NoError(t, err)
				data, err := idl.FetchImageData(context.TODO(), image)
				if err != nil {
					return nil, err
				}
				raw, err := json.Marshal(data.Data())
				if err != nil {
					return nil, err
				}
				var apiData map[string]any
				err = json.Unmarshal(raw, &apiData)
				if err != nil {
					return nil, err
				}
				return apiData, nil
			},
		},
		}}
	out, _, err := prog.Eval(data)
	assert.NoError(t, err)
	resolvedImg := out.Value().(string)
	assert.True(t, strings.HasPrefix(resolvedImg, "ghcr.io/kyverno/kyverno:latest@sha256:"))
}

func Test_impl_get_imagedata_string_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("image", ContextType),
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
		want: types.NewErr("expected 2 arguments, got %d", 0),
	}, {
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("foo")},
		want: types.NewErr("unsupported native conversion from string to 'imagedata.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false)},
		want: types.NewErr("type conversion error from bool to 'string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.get_imagedata_string(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}
