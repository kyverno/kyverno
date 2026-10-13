package engine

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

type payloadRequestMarker struct{}

const payloadPredicateType = "https://example.test/attestation"

type requestPayloadCache struct {
	imageverifycache.Client
}

func (requestPayloadCache) GetWithPayload(ctx context.Context, _ metav1.Object, _ string, _ string, _ bool) (bool, map[string][]byte, error) {
	payload, err := json.Marshal(map[string]any{"request": ctx.Value(payloadRequestMarker{})})
	return true, map[string][]byte{payloadPredicateType: payload}, err
}

type payloadBarrierImageContext struct {
	imagedataloader.ImageContext
	entered chan<- struct{}
	release <-chan struct{}
}

func (c *payloadBarrierImageContext) Get(ctx context.Context, _ string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	// extractPayload reaches this only after its request's attestation cache
	// hit has stored the deferred payload. Hold both requests here so any
	// accidentally shared pending entry would be overwritten or consumed.
	c.entered <- struct{}{}
	select {
	case <-c.release:
		return &imagedataloader.ImageData{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestHandleValidating_ConcurrentRequestsKeepDeferredPayloadsSeparate(t *testing.T) {
	policy := ivpol.DeepCopy()
	policy.Spec.MatchConstraints.ResourceRules[0].Resources = []string{"configmaps"}
	policy.Spec.MatchImageReferences = []policiesv1beta1.MatchImageReference{{Glob: "registry.example/*"}}
	policy.Spec.ValidationConfigurations = policiesv1alpha1.ValidationConfiguration{
		MutateDigest: ptr.To(false), VerifyDigest: ptr.To(false), Required: ptr.To(false),
	}
	policy.Spec.Attestations = []policiesv1beta1.Attestation{{
		Name: "provenance", InToto: &policiesv1beta1.InToto{Type: payloadPredicateType},
	}}
	policy.Spec.Validations = []admissionregistrationv1.Validation{{
		Expression: `verifyAttestationSignatures("registry.example/app:latest", "provenance", [attestors.notary]) == 1 && extractPayload("registry.example/app:latest", "provenance").request == object.data.request`,
	}}
	provider, err := NewProvider([]policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, requestPayloadCache{}, config.NewDefaultConfiguration(false)).(*engineImpl)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return &payloadBarrierImageContext{entered: entered, release: release}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type outcome struct {
		response eval.ImageVerifyEngineResponse
		err      error
	}
	outcomes := make(chan outcome, 2)
	for _, marker := range []string{"first", "second"} {
		raw, err := json.Marshal(map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{"name": marker, "namespace": "default"},
			"data":     map[string]any{"request": marker},
		})
		require.NoError(t, err)
		request := EngineRequest{
			Request: admissionv1.AdmissionRequest{
				Operation:       admissionv1.Create,
				Kind:            metav1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				Resource:        metav1.GroupVersionResource{Version: "v1", Resource: "configmaps"},
				RequestResource: &metav1.GroupVersionResource{Version: "v1", Resource: "configmaps"},
				Namespace:       "default", Name: marker,
				Object: runtime.RawExtension{Raw: raw},
			},
			Context: libs.NewFakeContextProvider(),
		}
		go func() {
			response, err := eng.HandleValidating(context.WithValue(ctx, payloadRequestMarker{}, marker), request, nil)
			outcomes <- outcome{response, err}
		}()
	}
	for range 2 {
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal("both requests must reach payload extraction before either continues")
		}
	}
	close(release)
	for range 2 {
		result := <-outcomes
		require.NoError(t, result.err)
		require.Len(t, result.response.Policies, 1)
		rule := result.response.Policies[0].Result
		require.Equal(t, engineapi.RuleStatusPass, rule.Status(), rule.Message())
	}
}
