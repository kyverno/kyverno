package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"

	"github.com/google/go-containerregistry/pkg/name"
	remote "github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
)

// writingImageContext stands in for the real image context. Its Get returns a
// cached *ImageData and then mutates it the way the cosign/notary verifiers do
// (ImageData.AddVerifiedIntotoPayloads, pkg/image/verifiers/ivpol/cosign/verifier.go:185).
// The shared barrier releases both policy goroutines at once, so writing a shared
// *ImageData is a guaranteed data race. A fresh context, and so a fresh
// *ImageData, is handed out on every call, so the writes collide only if the
// engine shares one context across the per-policy goroutines.
type writingImageContext struct {
	data    *imagedataloader.ImageData
	started chan struct{}
	release chan struct{}
}

func (writingImageContext) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return nil
}

func (c *writingImageContext) Get(context.Context, string, []remote.Option, []name.Option) (*imagedataloader.ImageData, error) {
	c.started <- struct{}{}
	<-c.release
	for i := 0; i < 200; i++ {
		c.data.AddVerifiedIntotoPayloads("https://slsa.dev/provenance/v1", []byte{1})
	}
	return c.data, nil
}

// Test_ImageVerifyEngine_ConcurrentEvaluationDoesNotShareImageData fails under
// -race with a data race on the shared *ImageData when the engine shares one
// image context across the per-policy goroutines, because two policies verifying
// the same image mutate that data at once. In production that same unsynchronized
// write can surface as a "fatal error: concurrent map writes" process crash. It
// passes when each policy goroutine gets its own image context. CI runs the unit
// tests with -race (Makefile).
func sameImageIvpol(n string) *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: n},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
					},
				}},
			},
			EvaluationConfiguration:  &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeKubernetes},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false)},
			MatchImageReferences:     []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			ImageExtractors:          []policiesv1beta1.ImageExtractor{{Name: "containers", Expression: "object.spec.containers.map(e, e.image)"}},
			Validations:              []admissionregistrationv1.Validation{{Expression: "true", Message: "x"}},
		},
	}
}

func Test_ImageVerifyEngine_ConcurrentEvaluationDoesNotShareImageData(t *testing.T) {
	provider := multiPolicyProvider(sameImageIvpol("ivpol-a"), sameImageIvpol("ivpol-b"))

	req := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Create,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          apiruntime.RawExtension{Raw: []byte(pod)},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			DryRun:          ptr.To(false),
		},
		Context: libs.NewFakeContextProvider(),
	}

	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)

	// one shared barrier; a fresh *ImageData per context
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return &writingImageContext{data: &imagedataloader.ImageData{}, started: started, release: release}, nil
	}

	old := libs.LibraryContext
	libs.LibraryContext = libs.NewFakeContextProvider()
	defer func() { libs.LibraryContext = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type outcome struct {
		resp eval.ImageVerifyEngineResponse
		err  error
	}
	resCh := make(chan outcome, 1)
	go func() {
		resp, err := eng.HandleValidating(ctx, req, nil)
		resCh <- outcome{resp, err}
	}()

	// wait until both policies are inside Get, then release them together so the
	// two writes overlap
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d of 2 policies reached image evaluation", i)
		}
	}
	close(release)

	select {
	case out := <-resCh:
		assert.NoError(t, out.err)
		// Both policies must produce their own deterministic result. A per-policy
		// error recorded in the response would leave HandleValidating returning no
		// error, so assert the status rather than only the top-level error. These
		// trivial policies perform no signature or attestation check, so the
		// required-verification gate fails both; the point is that the result is a
		// clean Fail for each, not an evaluation error or a dropped policy.
		statuses := map[string]engineapi.RuleStatus{}
		for _, p := range out.resp.Policies {
			statuses[p.Policy.GetName()] = p.Result.Status()
		}
		assert.Len(t, out.resp.Policies, 2)
		assert.Equal(t, engineapi.RuleStatusFail, statuses["ivpol-a"])
		assert.Equal(t, engineapi.RuleStatusFail, statuses["ivpol-b"])
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not complete")
	}
}

// Test_ImageVerifyEngine_ImageContextConstructionErrorFailsRequest pins the error
// contract: when the image context cannot be built, the whole request fails (the
// webhook's failure policy then decides) rather than the error being downgraded
// to a per-policy result, which for a Warn policy would admit.
func Test_ImageVerifyEngine_ImageContextConstructionErrorFailsRequest(t *testing.T) {
	provider := multiPolicyProvider(sameImageIvpol("ivpol-a"), sameImageIvpol("ivpol-b"))

	req := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Create,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          apiruntime.RawExtension{Raw: []byte(pod)},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			DryRun:          ptr.To(false),
		},
		Context: libs.NewFakeContextProvider(),
	}

	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return nil, fmt.Errorf("image context unavailable")
	}

	old := libs.LibraryContext
	libs.LibraryContext = libs.NewFakeContextProvider()
	defer func() { libs.LibraryContext = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := eng.HandleValidating(ctx, req, nil)
	assert.Error(t, err)
}
