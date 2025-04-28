package imagedata

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/stretchr/testify/assert"
)

func Test_impl_get_imagedata_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("image", ContextType),
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
		"image": Context{&resource.MockCtx{
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
