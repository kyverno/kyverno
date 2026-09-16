package engine

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen/extract"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"
)

// jobSetWithImage mirrors vpol's fixture of the same name (same shape as the
// repro used against issue #16477): a minimal JobSet with one pod-template
// container.
func jobSetWithImage(image string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "latest-tag-jobset", "namespace": "default"},
		"spec": map[string]any{
			"replicatedJobs": []any{
				map[string]any{
					"name": "workers",
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{"name": "worker", "image": image},
									},
								},
							},
						},
					},
				},
			},
		},
	}}
}

// buildExtractionModeRequestPolicy builds a Pod-targeted ImageValidatingPolicy,
// left completely unmodified, with autogen configured for a custom CRD via
// AutogenConfiguration.PodControllers (see pkg/cel/policies/ivpol/autogen).
// Its validation reads request.object instead of the top-level object
// variable, so it only passes if extraction mode synthesizes a matching
// AdmissionRequest -- not just matching admission attributes -- for each pod
// template.
func buildExtractionModeRequestPolicy() *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "extraction-request-object"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeKubernetes,
			},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(false),
				VerifyDigest: ptr.To(false),
				Required:     ptr.To(false),
			},
			AutogenConfiguration: &policiesv1beta1.ImageValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"jobsets.v1alpha2.jobset.x-k8s.io"},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "request.kind.kind == 'Pod' && request.object.spec.containers.all(c, !c.image.endsWith(':latest'))"},
			},
		},
	}
}

// TestHandleValidating_ExtractionMode_RequestObjectMatchesSynthesizedPod
// proves each synthesized pod-template request built by evaluateExtractedIv
// carries its own request.object (the synthesized Pod), not the outer
// JobSet's -- the carve-out prepareK8sData's doc comment mandates. The
// single-assembly-per-call guarantee that keeps this carve-out cheap (one
// build per template, not one per matchCondition/exception) is proven at the
// evaluator level by
// TestEvaluate_MatchConditionsExceptionsAndValidationShareOneHoistedMap in
// pkg/image/verification/evaluator.
func TestHandleValidating_ExtractionMode_RequestObjectMatchesSynthesizedPod(t *testing.T) {
	policy := buildExtractionModeRequestPolicy()
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	// The validation only reads request.object, but the default built-in image
	// extractors still pick up the container image and the pre-validation
	// prefetch loop in Evaluate resolves it - stub that out so the test does
	// not depend on network access, matching
	// TestHandleValidatingEphemeralContainersWithImagesVariable's fakeImageContext use.
	eng.newImageContext = func() (imagedataloader.ImageContext, error) {
		return fakeImageContext{}, nil
	}

	assertSingleRuleResult := func(t *testing.T, image string) *engineapi.RuleResponse {
		t.Helper()
		obj := jobSetWithImage(image)
		raw, err := obj.MarshalJSON()
		require.NoError(t, err)
		req := engine.EngineRequest{
			Request: v1.AdmissionRequest{
				Operation:       v1.Create,
				Kind:            metav1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
				Resource:        metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
				RequestResource: &metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
				Namespace:       "default",
				Name:            "latest-tag-jobset",
				Object:          apiruntime.RawExtension{Raw: raw},
			},
		}
		resp, err := eng.HandleValidating(context.Background(), req, nil)
		require.NoError(t, err)
		require.Len(t, resp.Policies, 1, "only the extraction-mode JobSet target should have matched and produced a result")
		return &resp.Policies[0].Result
	}

	t.Run("request.object sees the synthesized Pod, not the JobSet - bad image is denied", func(t *testing.T) {
		rule := assertSingleRuleResult(t, "bash:latest")
		require.NotEqual(t, engineapi.RuleStatusError, rule.Status(), "message: %s", rule.Message())
		assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	})

	t.Run("request.object sees the synthesized Pod, not the JobSet - compliant image is allowed", func(t *testing.T) {
		rule := assertSingleRuleResult(t, "bash:1.0")
		require.NotEqual(t, engineapi.RuleStatusError, rule.Status(), "message: %s", rule.Message())
		assert.Equal(t, engineapi.RuleStatusPass, rule.Status())
	})
}

// jobSetWithNTemplates builds a JobSet with n independent replicatedJobs
// entries, each containing exactly one pod template -- so
// extract.ExtractPodTemplates(obj) finds exactly n templates.
func jobSetWithNTemplates(n int) *unstructured.Unstructured {
	jobs := make([]any, 0, n)
	for i := 0; i < n; i++ {
		jobs = append(jobs, map[string]any{
			"name": fmt.Sprintf("workers-%d", i),
			"template": map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{"name": "worker", "image": "bash:1.0"},
							},
						},
					},
				},
			},
		})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "multi-template-jobset", "namespace": "default"},
		"spec":       map[string]any{"replicatedJobs": jobs},
	}}
}

// countingNilThunkCompiledPolicy is a stub eval.CompiledPolicy whose Evaluate
// records how many times it is called and asserts, on every call, that the
// requestMapFn it received is nil -- the ExtractionMode carve-out
// prepareK8sData's doc comment mandates (each synthesized pod template must
// build its own map, never reuse the outer request's hoisted one).
type countingNilThunkCompiledPolicy struct {
	t                testing.TB
	calls            atomic.Int32
	sawNonNilRequest atomic.Bool
}

func (c *countingNilThunkCompiledPolicy) Evaluate(
	_ context.Context,
	_ *imageverify.Runtime,
	_ admission.Attributes,
	_ interface{},
	_ apiruntime.Object,
	_ bool,
	requestMapFn func() (map[string]any, error),
	_ libs.Context,
) (*eval.EvaluationResult, error) {
	c.calls.Add(1)
	if requestMapFn != nil {
		c.sawNonNilRequest.Store(true)
	}
	return &eval.EvaluationResult{Result: true}, nil
}

func (c *countingNilThunkCompiledPolicy) EnforceRequired(images []string, _ *imageverify.ImageVerificationResults) error {
	return nil
}

func (c *countingNilThunkCompiledPolicy) MutateDigest(
	context.Context,
	*imageverify.Runtime,
	admission.Attributes,
	interface{},
	apiruntime.Object,
	unstructured.Unstructured,
	func() (map[string]any, error),
	config.Configuration,
	libs.Context,
) ([]jsonpatch.JsonPatchOperation, error) {
	c.t.Fatal("MutateDigest must not be called by evaluateExtractedIv")
	return nil, nil
}

// TestEvaluateExtractedIv_BuildsExactlyOncePerTemplate_NeverSharesOuterThunk
// is the extraction-mode build-count case the design's test plan calls for:
// it drives evaluateExtractedIv directly (bypassing image extraction/network)
// against a CRD with T=3 pod templates and asserts exactly T calls to
// CompiledPolicy.Evaluate -- not T x (2 + exceptions), the multiplication the
// adversarial review flagged (MAJOR 1) -- and that every one of those calls
// received a nil requestMapFn, proving evaluateExtractedIv never leaks the
// outer request's hoisted thunk into a synthesized per-template evaluation.
func TestEvaluateExtractedIv_BuildsExactlyOncePerTemplate_NeverSharesOuterThunk(t *testing.T) {
	const numTemplates = 3
	source := jobSetWithNTemplates(numTemplates)
	// sanity: confirm the fixture actually yields numTemplates templates
	// before asserting on the call count that depends on it.
	require.Len(t, extract.ExtractPodTemplates(source.Object), numTemplates)

	oldObj := &unstructured.Unstructured{}
	attr := admission.NewAttributesRecord(
		source,
		oldObj,
		source.GroupVersionKind(),
		source.GetNamespace(),
		source.GetName(),
		schema.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
		"",
		admission.Create,
		nil,
		false,
		nil,
	)
	request := &v1.AdmissionRequest{
		Operation: v1.Create,
		Kind:      metav1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
		Resource:  metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
		Namespace: "default",
		Name:      "multi-template-jobset",
	}

	compiled := &countingNilThunkCompiledPolicy{t: t}
	e := &engineImpl{}
	result, err := e.evaluateExtractedIv(context.Background(), compiled, &imageverify.Runtime{ImageContext: fakeImageContext{}}, attr, request, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)

	assert.Equal(t, int32(numTemplates), compiled.calls.Load(),
		"evaluateExtractedIv must call Evaluate exactly once per pod template, not once per (matchConditions + exceptions) per template")
	assert.False(t, compiled.sawNonNilRequest.Load(),
		"evaluateExtractedIv must never pass the outer request's hoisted requestMapFn into a synthesized per-template Evaluate call")
}
