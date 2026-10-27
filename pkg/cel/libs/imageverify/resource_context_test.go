package imageverify

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type acceptingVerificationCache struct {
	imageverifycache.Client
	calls int
}

func (cache *acceptingVerificationCache) Get(context.Context, metav1.Object, string, string, bool) (bool, error) {
	cache.calls++
	return true, nil
}

func (cache *acceptingVerificationCache) GetWithPayload(context.Context, metav1.Object, string, string, bool) (bool, map[string][]byte, error) {
	cache.calls++
	return true, nil, nil
}

func TestResourceVerificationCacheCannotBypassDestinationCheck(t *testing.T) {
	for _, expression := range []string{
		`verifyImageSignatures(image, [attestors.notary])`,
		`verifyAttestationSignatures(image, "sbom", [attestors.notary])`,
	} {
		for _, resource := range []bool{false, true} {
			label := "author"
			if resource {
				label = "resource"
			}
			t.Run(expression+"/"+label, func(t *testing.T) {
				const image = "127.0.0.1:5000/app@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
				images := &recordingImageContext{}
				refs := []string{"127.0.0.1:5000/another:latest"}
				if resource {
					refs = []string{image}
				}
				opts, names := registryclient.ResourceImageOptions()
				scoped, err := NewResourceImageContext(images, refs, opts, names)
				require.NoError(t, err)
				cache := &acceptingVerificationCache{}
				env, err := cel.NewEnv(cel.Variable("image", cel.StringType),
					cel.Variable("attestors", cel.MapType(cel.StringType, cel.DynType)),
					Lib(nil, images, ivpol, nil, logr.Discard(), cache, NewImageVerificationResults()))
				require.NoError(t, err)
				ast, issues := env.Compile(expression)
				require.Nil(t, issues)
				program, err := env.Program(ast)
				require.NoError(t, err)
				_, _, err = program.Eval(map[string]any{
					"image":           image,
					"attestors":       map[string]policiesv1beta1.Attestor{"notary": ivpol.Spec.Attestors[0]},
					RequestContextKey: NewEvaluationContext(context.Background(), scoped),
				})
				if resource {
					require.ErrorContains(t, err, "connection blocked to network host")
					require.Zero(t, cache.calls)
				} else {
					require.NoError(t, err)
					require.Equal(t, 1, cache.calls)
				}
				require.Empty(t, images.contexts)
			})
		}
	}
}
