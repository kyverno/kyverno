package imageverify

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type runtimeImages struct {
	image *imagedataloader.ImageData
	err   error
}

func (r runtimeImages) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return nil
}
func (r runtimeImages) Get(context.Context, string, []remote.Option, []name.Option) (*imagedataloader.ImageData, error) {
	return r.image, r.err
}

func TestReusableProgramsIsolateRuntime(t *testing.T) {
	env, err := cel.NewEnv(Lib(), cel.Variable("attestors", cel.ListType(cel.DynType)), cel.Variable("expected", cel.StringType))
	require.NoError(t, err)
	compile := func(expression string) cel.Program {
		ast, issues := env.Compile(expression)
		require.NoError(t, issues.Err())
		program, err := env.Program(ast)
		require.NoError(t, err)
		return program
	}
	signature := compile(`verifyImageSignatures("image", attestors)`)
	attestation := compile(`verifyAttestationSignatures("image", "proof", attestors)`)
	payload := compile(`extractPayload("image", "proof").request == expected`)
	metadata := compile(`getImageData("image")`)
	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "runtime", UID: "runtime", ResourceVersion: "1"},
		Spec:       policiesv1beta1.ImageValidatingPolicySpec{Attestations: []policiesv1beta1.Attestation{{Name: "proof", InToto: &policiesv1beta1.InToto{Type: "proof"}}}},
	}
	factory := NewFactory(logr.Discard(), policy, nil, env.CELTypeAdapter(), nil)
	attestors := []policiesv1beta1.Attestor{{Name: "test"}}
	for i := range 32 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			expected := fmt.Sprint(i)
			cache, err := imageverifycache.New(imageverifycache.WithCacheEnableFlag(true), imageverifycache.WithMaxSize(0), imageverifycache.WithTTLDuration(0))
			require.NoError(t, err)
			if i%2 == 0 {
				stored, err := cache.Set(context.Background(), policy, attestorCacheRule(signatureCacheRule, "", attestors), "image", true)
				require.NoError(t, err)
				require.True(t, stored)
			}
			stored, err := cache.SetWithPayload(context.Background(), policy, attestorCacheRule(attestationCacheRule, "proof", attestors), "image", true, map[string][]byte{"proof": []byte(fmt.Sprintf(`{"request":%q}`, expected))})
			require.NoError(t, err)
			require.True(t, stored)
			images := runtimeImages{image: &imagedataloader.ImageData{}}
			images.image.Digest = expected
			results := NewImageVerificationResults()
			runtime := factory.Bind(&Runtime{ImageContext: images, Cache: cache, Results: results})
			activation := map[string]any{RuntimeKey: runtime, "attestors": attestors, "expected": expected}
			_, _, err = metadata.Eval(map[string]any{RuntimeKey: factory.Bind(&Runtime{ImageContext: runtimeImages{err: fmt.Errorf("request %s", expected)}})})
			require.ErrorContains(t, err, "request "+expected)
			var out ref.Val
			out, _, err = signature.Eval(activation)
			require.NoError(t, err)
			require.Equal(t, int64(1-i%2), out.Value())
			verified, attempted := results.Status("image")
			require.True(t, attempted)
			require.Equal(t, i%2 == 0, verified)
			_, _, err = attestation.Eval(activation)
			require.NoError(t, err)
			out, _, err = payload.Eval(activation)
			require.NoError(t, err)
			require.Equal(t, true, out.Value())
			require.Empty(t, runtime.functions.pendingIntotoRestores)
			require.Empty(t, factory.functions.pendingIntotoRestores)
		})
	}
}

func TestReusableProgramRequiresRuntime(t *testing.T) {
	t.Parallel()
	env, err := cel.NewEnv(Lib())
	require.NoError(t, err)
	ast, issues := env.Compile(`getImageData("image")`)
	require.NoError(t, issues.Err())
	program, err := env.Program(ast)
	require.NoError(t, err)
	_, _, err = program.Eval(map[string]any{})
	require.Error(t, err)
	_, _, err = program.Eval(map[string]any{RuntimeKey: Runtime{}})
	require.ErrorContains(t, err, "missing image verification runtime")
}
