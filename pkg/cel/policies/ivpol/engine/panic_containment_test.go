package engine

import (
	"context"
	"testing"
	"time"

	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/imagedataloader"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
)

// panickingCompiledPolicy is a stub CompiledPolicy whose Evaluate panics.
// It exercises the recover in evaluatePolicies directly without running into
// cel-go's internal recover, which intercepts panics raised inside CEL functions.
type panickingCompiledPolicy struct {
	panicMsg string
}

func (p panickingCompiledPolicy) Evaluate(context.Context, imagedataloader.ImageContext, imageverifycache.Client, *imageverify.ImageVerificationResults, admission.Attributes, interface{}, apiruntime.Object, bool, func() (map[string]any, error), libs.Context) (*iveval.EvaluationResult, error) {
	panic(p.panicMsg)
}

func (p panickingCompiledPolicy) EnforceRequired([]string, *imageverify.ImageVerificationResults) error {
	return nil
}

// Tracing reports false: the stub is never compiled for decision tracing.
func (p panickingCompiledPolicy) Tracing() bool { return false }

func (p panickingCompiledPolicy) MutateDigest(context.Context, imagedataloader.ImageContext, imageverifycache.Client, *imageverify.ImageVerificationResults, admission.Attributes, interface{}, apiruntime.Object, unstructured.Unstructured, func() (map[string]any, error), config.Configuration, libs.Context) ([]jsonpatch.JsonPatchOperation, error) {
	return nil, nil
}

type panickingCompiler struct {
	panicMsg string
}

func (c panickingCompiler) Compile(policiesv1beta1.ImageValidatingPolicyLike, []*policiesv1beta1.PolicyException) (iveval.CompiledPolicy, field.ErrorList) {
	return panickingCompiledPolicy{panicMsg: c.panicMsg}, nil
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
	panickingPolicy := globIvpol("ivpol-panics", "ghcr.io/a/*")
	healthyPolicy := globIvpol("ivpol-healthy", "ghcr.io/b/*")

	healthyCompiled, errs := iveval.NewCompiler(nil).Compile(healthyPolicy, nil)
	require.Empty(t, errs)

	provider := ProviderFunc(func(context.Context) ([]Policy, error) {
		return []Policy{
			{
				Policy:         panickingPolicy,
				CompiledPolicy: panickingCompiledPolicy{panicMsg: "injected panic while fetching ghcr.io/a/app:v1"},
				Actions:        sets.New(admissionregistrationv1.Deny),
			},
			{
				Policy:         healthyPolicy,
				CompiledPolicy: healthyCompiled,
				Actions:        sets.New(admissionregistrationv1.Deny),
			},
		}, nil
	})

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
	pol := buildExtractionModeRequestPolicy()
	provider, err := NewProvider(panickingCompiler{panicMsg: "injected panic while fetching ghcr.io/a/app:v1"}, []policiesv1beta1.ImageValidatingPolicyLike{pol}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	const image = "ghcr.io/a/app:v1"

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
