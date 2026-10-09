package imageverify

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type requestMarker struct{}

// recordingImageContext records the context each image fetch is called with and
// fails the fetch, so the CEL function stops right after it.
type recordingImageContext struct {
	mu   sync.Mutex
	ctxs []context.Context
}

func (c *recordingImageContext) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return nil
}

func (c *recordingImageContext) Get(ctx context.Context, _ string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ctxs = append(c.ctxs, ctx)
	return nil, errors.New("fetch stopped by test")
}

// Test_ImageFunctionsUseTheAdmissionRequestContext: the image functions fetch
// images and verify them while an admission request is waiting, so they must
// use that request's context. With context.TODO() a cancelled or timed-out
// admission request keeps its registry and verification work running.
func Test_ImageFunctionsUseTheAdmissionRequestContext(t *testing.T) {
	env, err := cel.NewEnv(cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)), Lib())
	require.NoError(t, err)
	attestors := map[string]v1beta1.Attestor{
		"notary": {Name: "notary", Notary: &v1beta1.Notary{Certs: &v1beta1.StringOrExpression{Value: cert}}},
	}
	for _, expression := range []string{
		`getImageData("ghcr.io/a/app:v1")`,
		`verifyImageSignatures("ghcr.io/a/app:v1", [attestors.notary])`,
		`verifyAttestationSignatures("ghcr.io/a/app:v1", "sbom", [attestors.notary])`,
		`extractPayload("ghcr.io/a/app:v1", "sbom")`,
	} {
		t.Run(expression, func(t *testing.T) {
			ast, issues := env.Compile(expression)
			require.Nil(t, issues)
			prog, err := env.Program(ast)
			require.NoError(t, err)

			imgCtx := &recordingImageContext{}
			requestCtx := context.WithValue(context.Background(), requestMarker{}, "admission-request")
			runtime := NewRuntimeForPolicy(requestCtx, NewIvFuncs(logr.Discard(), ivpol, nil, env.CELTypeAdapter(), nil), imgCtx, nil, NewImageVerificationResults())
			_, _, _ = prog.Eval(map[string]any{RuntimeKey: runtime, "attestors": attestors})

			require.NotEmpty(t, imgCtx.ctxs, "the function never fetched the image")
			for _, ctx := range imgCtx.ctxs {
				assert.Equal(t, "admission-request", ctx.Value(requestMarker{}))
			}
		})
	}
}
