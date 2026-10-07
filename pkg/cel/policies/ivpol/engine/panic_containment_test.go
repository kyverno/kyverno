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
	"github.com/stretchr/testify/require"

	"github.com/google/go-containerregistry/pkg/name"
	remote "github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
)

// panickingImageContext panics when asked for one specific image and returns
// empty image data for any other image. It stands in for any Go panic raised
// while a policy evaluates outside cel-go (here at the image fetch boundary),
// which cel-go's own recover does not cover.
type panickingImageContext struct {
	panicOn string
}

func (panickingImageContext) AddImages(context.Context, []string, []remote.Option, []name.Option) error {
	return nil
}

func (c panickingImageContext) Get(_ context.Context, image string, _ []remote.Option, _ []name.Option) (*imagedataloader.ImageData, error) {
	if image == c.panicOn {
		panic(fmt.Sprintf("injected panic while fetching %s", image))
	}
	return &imagedataloader.ImageData{}, nil
}

// globIvpol is a pods-only ImageValidatingPolicy that matches images by glob,
// so two policies can be pointed at different images of the same Pod.
func globIvpol(n, glob string) *policiesv1beta1.ImageValidatingPolicy {
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
			MatchImageReferences:     []policiesv1beta1.MatchImageReference{{Glob: glob}},
			ImageExtractors:          []policiesv1beta1.ImageExtractor{{Name: "containers", Expression: "object.spec.containers.map(e, e.image)"}},
			Validations:              []admissionregistrationv1.Validation{{Expression: "true", Message: "x"}},
		},
	}
}

const twoImagePod = `{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {"name": "web", "namespace": "default"},
	"spec": {
		"containers": [
			{"name": "app", "image": "ghcr.io/a/app:v1"},
			{"name": "sidecar", "image": "ghcr.io/b/proxy:v1"}
		]
	}
}`

// Test_ImageVerifyEngine_PanicInOnePolicyDoesNotCrashController: when one
// ImageValidatingPolicy panics during evaluation, the admission request must
// still complete. Before policies were evaluated concurrently the panic ran on
// the HTTP serving goroutine, where net/http recovered it for that request
// only. With one goroutine per policy, an unrecovered panic in any of them
// terminates the whole admission controller.
func Test_ImageVerifyEngine_PanicInOnePolicyDoesNotCrashController(t *testing.T) {
	provider := multiPolicyProvider(
		globIvpol("ivpol-panics", "ghcr.io/a/*"),
		globIvpol("ivpol-healthy", "ghcr.io/b/*"),
	)

	req := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Create,
			Kind:            metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object:          apiruntime.RawExtension{Raw: []byte(twoImagePod)},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			DryRun:          ptr.To(false),
		},
		Context: libs.NewFakeContextProvider(),
	}

	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return panickingImageContext{panicOn: "ghcr.io/a/app:v1"}, nil
	}

	old := libs.LibraryContext
	libs.LibraryContext = libs.NewFakeContextProvider()
	defer func() { libs.LibraryContext = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// An unrecovered panic in a policy goroutine aborts the test binary here.
	resp, err := eng.HandleValidating(ctx, req, nil)

	// The panic stays inside the policy that raised it: the request itself does
	// not fail, the panicking policy reports an evaluation error carrying the
	// panic value, and the other policy keeps its own verdict.
	assert.NoError(t, err)
	assert.Len(t, resp.Policies, 2)
	statuses := map[string]engineapi.RuleStatus{}
	messages := map[string]string{}
	for _, p := range resp.Policies {
		statuses[p.Policy.GetName()] = p.Result.Status()
		messages[p.Policy.GetName()] = p.Result.Message()
	}
	assert.Equal(t, engineapi.RuleStatusError, statuses["ivpol-panics"])
	assert.Contains(t, messages["ivpol-panics"], "injected panic while fetching ghcr.io/a/app:v1")
	// These policies perform no signature or attestation check, so the
	// required-verification gate fails the healthy one, as it would without the
	// panicking policy next to it.
	assert.Equal(t, engineapi.RuleStatusFail, statuses["ivpol-healthy"])
}

// Test_ImageVerifyEngine_PanicInExtractionModePolicyIsContained: autogen'd
// targets whose pod templates are evaluated in extraction mode
// (evaluateExtractedIv) run in the same per-policy goroutine, so a panic there
// is contained the same way.
func Test_ImageVerifyEngine_PanicInExtractionModePolicyIsContained(t *testing.T) {
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{buildExtractionModeRequestPolicy()}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	const image = "ghcr.io/a/app:v1"
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return panickingImageContext{panicOn: image}, nil
	}

	raw, err := jobSetWithImage(image).MarshalJSON()
	require.NoError(t, err)
	req := engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Create,
			Kind:            metav1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
			Resource:        metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
			RequestResource: &metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
			Namespace:       "default",
			Name:            "web",
			Object:          apiruntime.RawExtension{Raw: raw},
		},
	}

	resp, err := eng.HandleValidating(context.Background(), req, nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	result := resp.Policies[0].Result
	assert.Equal(t, engineapi.RuleStatusError, result.Status())
	assert.Contains(t, result.Message(), "injected panic while fetching ghcr.io/a/app:v1")
}
