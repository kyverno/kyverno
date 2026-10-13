package imageverify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
)

type requestMarker struct{}

// recordingImageContext stops at the loader boundary, without registry traffic.
type recordingImageContext struct {
	imagedataloader.ImageContext
	contexts []context.Context
	data     *imagedataloader.ImageData
}

func (c *recordingImageContext) Get(ctx context.Context, _ string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	c.contexts = append(c.contexts, ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.data != nil {
		return c.data, nil
	}
	return nil, errors.New("fetch stopped by test")
}

func TestImageFunctionsUseRequestContext(t *testing.T) {
	t.Parallel()
	for _, expression := range []string{
		`getImageData("registry.example/app:latest")`,
		`verifyImageSignatures("registry.example/app:latest", [attestors.notary])`,
		`verifyAttestationSignatures("registry.example/app:latest", "sbom", [attestors.notary])`,
		`extractPayload("registry.example/app:latest", "sbom")`,
	} {
		t.Run(expression, func(t *testing.T) {
			t.Parallel()
			images := &recordingImageContext{}
			env, err := cel.NewEnv(
				cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
				Lib(nil, images, ivpol, nil, logr.Discard(), nil, NewImageVerificationResults()),
			)
			require.NoError(t, err)
			ast, issues := env.Compile(expression)
			require.Nil(t, issues)
			program, err := env.Program(ast)
			require.NoError(t, err)
			data := map[string]any{"attestors": map[string]policiesv1beta1.Attestor{"notary": ivpol.Spec.Attestors[0]}}

			first, cancelFirst := context.WithTimeout(context.WithValue(context.Background(), requestMarker{}, "first"), time.Minute)
			defer cancelFirst()
			second, cancelSecond := context.WithTimeout(context.WithValue(context.Background(), requestMarker{}, "second"), 2*time.Minute)
			defer cancelSecond()
			for _, ctx := range []context.Context{first, second} {
				data[RequestContextKey] = NewEvaluationContext(ctx)
				_, _, err := program.Eval(data)
				require.ErrorContains(t, err, "fetch stopped by test")
				require.Equal(t, ctx, images.contexts[len(images.contexts)-1])
				deadline, _ := ctx.Deadline()
				observed, ok := images.contexts[len(images.contexts)-1].Deadline()
				require.True(t, ok)
				require.Equal(t, deadline, observed)
				// Canceling the first evaluation must not affect the next one.
				cancelFirst()
			}
			require.Len(t, images.contexts, 2)

			data[RequestContextKey] = NewEvaluationContext(first)
			_, _, err = program.Eval(data)
			require.ErrorContains(t, err, context.Canceled.Error())
			expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancelExpired()
			data[RequestContextKey] = NewEvaluationContext(expired)
			_, _, err = program.Eval(data)
			require.ErrorContains(t, err, context.DeadlineExceeded.Error())

			// Existing standalone expressions need no internal activation variable.
			delete(data, RequestContextKey)
			_, _, err = program.Eval(data)
			require.ErrorContains(t, err, "fetch stopped by test")
			require.Len(t, images.contexts, 5)
			standalone := images.contexts[4]
			require.NoError(t, standalone.Err())
			require.Nil(t, standalone.Value(requestMarker{}))
			_, ok := standalone.Deadline()
			require.False(t, ok)
			_, issues = env.Compile(RequestContextKey + ".ctx")
			require.Error(t, issues.Err(), "request internals must not be exposed to CEL")
			data[RequestContextKey] = EvaluationContext{}
			_, _, err = program.Eval(data)
			require.ErrorContains(t, err, "missing image verification evaluation context")
			require.Len(t, images.contexts, 5, "an empty receiver must not silently select a background context")
			direct, issues := env.Compile("imageverify.EvaluationContext{}.__" + expression)
			require.Nil(t, issues)
			directProgram, err := env.Program(direct)
			require.NoError(t, err)
			_, _, err = directProgram.Eval(data)
			require.ErrorContains(t, err, "missing image verification evaluation context")
			require.Len(t, images.contexts, 5, "CEL-constructed receivers must not start image requests")
		})
	}
}

func TestImageFunctionsShareCachedVerificationAndPayload(t *testing.T) {
	t.Parallel()
	for _, activationContext := range []bool{false, true} {
		name := "standalone"
		if activationContext {
			name = "evaluation context"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			const image = "registry.example/app:latest"
			const predicate = "https://example.com/attestation"
			policy := ivpol.DeepCopy()
			policy.Name = "cached-payload"
			policy.Spec.Attestors = []policiesv1beta1.Attestor{{Name: "cached", Cosign: &policiesv1beta1.Cosign{}}}
			policy.Spec.Attestations = []policiesv1beta1.Attestation{{Name: "sbom", InToto: &policiesv1beta1.InToto{Type: predicate}}}
			cache, err := imageverifycache.New(imageverifycache.WithCacheEnableFlag(true), imageverifycache.WithMaxSize(0), imageverifycache.WithTTLDuration(0))
			require.NoError(t, err)
			signatureRule := attestorCacheRule(signatureCacheRule, "", policy.Spec.Attestors)
			stored, err := cache.Set(context.Background(), policy, signatureRule, image, true)
			require.NoError(t, err)
			require.True(t, stored)
			attestationRule := attestorCacheRule(attestationCacheRule, "sbom", policy.Spec.Attestors)
			stored, err = cache.SetWithPayload(context.Background(), policy, attestationRule, image, true, map[string][]byte{predicate: []byte(`{"verified":"payload"}`)})
			require.NoError(t, err)
			require.True(t, stored)
			images := &recordingImageContext{data: &imagedataloader.ImageData{}}
			results := NewImageVerificationResults()
			env, err := cel.NewEnv(
				cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
				Lib(nil, images, policy, nil, logr.Discard(), cache, results),
			)
			require.NoError(t, err)
			ast, issues := env.Compile(`verifyImageSignatures("registry.example/app:latest", [attestors.cached]) == 1 &&
				verifyAttestationSignatures("registry.example/app:latest", "sbom", [attestors.cached]) == 1 &&
				extractPayload("registry.example/app:latest", "sbom").verified == "payload"`)
			require.Nil(t, issues)
			program, err := env.Program(ast)
			require.NoError(t, err)
			ctx := context.WithValue(context.Background(), requestMarker{}, name)
			data := map[string]any{"attestors": map[string]policiesv1beta1.Attestor{"cached": policy.Spec.Attestors[0]}}
			if activationContext {
				data[RequestContextKey] = NewEvaluationContext(ctx)
			}
			out, _, err := program.Eval(data)
			require.NoError(t, err)
			require.Equal(t, true, out.Value())
			verified, attempted := results.Status(image)
			require.True(t, verified)
			require.True(t, attempted)
			require.Len(t, images.contexts, 1, "cache hits must defer image loading until extractPayload")
			if activationContext {
				require.Equal(t, ctx, images.contexts[0])
			}
		})
	}
}
