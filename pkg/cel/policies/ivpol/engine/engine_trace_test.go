package engine

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// tracePolicy is a Kubernetes-mode policy on pods that checks every ghcr.io image without
// verifying any signature (so nothing reaches a registry), with autogen off so one policy yields
// one response.
func tracePolicy(validation string) *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "trace-ivpol"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
					},
				}},
			},
			AutogenConfiguration: &policiesv1beta1.ImageValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{Controllers: []string{}},
			},
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeKubernetes},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
				MutateDigest: ptr.To(false),
				VerifyDigest: ptr.To(false),
				Required:     ptr.To(false),
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "ghcr.io/*"}},
			Validations:          []admissionregistrationv1.Validation{{Expression: validation, Message: "validation failed"}},
		},
	}
}

func podRequest(kind, resource, namespace string) engine.EngineRequest {
	object := `{"apiVersion":"v1","kind":"` + kind + `","metadata":{"name":"web","namespace":"` + namespace + `"},` +
		`"spec":{"containers":[{"name":"app","image":"ghcr.io/x/app:1.0"},{"name":"sidecar","image":"docker.io/library/busybox:1"}]}}`
	return engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind:      metav1.GroupVersionKind{Version: "v1", Kind: kind},
			Resource:  metav1.GroupVersionResource{Version: "v1", Resource: resource},
			// the built-in image extractors are chosen by the requested resource
			RequestResource: &metav1.GroupVersionResource{Version: "v1", Resource: resource},
			Name:            "web",
			Namespace:       namespace,
			Object:          apiruntime.RawExtension{Raw: []byte(object)},
		},
		Context: libs.NewFakeContextProvider(),
	}
}

func handleTraced(t *testing.T, traced bool, policy *policiesv1beta1.ImageValidatingPolicy, request engine.EngineRequest, exceptions ...*policiesv1beta1.PolicyException) []iveval.ImageVerifyPolicyResponse {
	t.Helper()
	provider, err := NewProvider(iveval.NewCompilerWithTrace(nil, traced), []policiesv1beta1.ImageValidatingPolicyLike{policy}, exceptions)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
	eng.newImageContext = func() (imagedataloader.ImageContext, error) { return fakeImageContext{}, nil }
	resp, err := eng.HandleValidating(context.Background(), request, nil)
	require.NoError(t, err)
	return resp.Policies
}

func TestHandleValidating_Tracing(t *testing.T) {
	tests := []struct {
		name       string
		validation string
		request    engine.EngineRequest
		required   bool
		exception  bool
		// wantRule is the reported rule status, empty when no rule is reported at all
		wantRule    engineapi.RuleStatus
		wantApplied bool
		wantVerdict string
		wantMessage string
		wantImages  bool
	}{{
		name:        "passes",
		validation:  "images.containers.all(i, i.startsWith('ghcr.io/'))",
		request:     podRequest("Pod", "pods", "default"),
		wantRule:    engineapi.RuleStatusPass,
		wantApplied: true,
		wantVerdict: trace.VerdictPass,
		wantImages:  true,
	}, {
		name:        "fails",
		validation:  "images.containers.size() == 0",
		request:     podRequest("Pod", "pods", "default"),
		wantRule:    engineapi.RuleStatusFail,
		wantApplied: true,
		wantVerdict: trace.VerdictFail,
		wantMessage: "validation failed",
		wantImages:  true,
	}, {
		name:        "matchConstraints do not cover the resource",
		validation:  "true",
		request:     podRequest("ConfigMap", "configmaps", "default"),
		wantApplied: false,
		wantVerdict: trace.VerdictSkip,
		wantMessage: "the policy does not apply to this resource",
	}, {
		// the validations pass, but nothing verified the image, so required fails the policy
		name:        "required turns a pass into a failure",
		validation:  "true",
		request:     podRequest("Pod", "pods", "default"),
		required:    true,
		wantRule:    engineapi.RuleStatusFail,
		wantApplied: true,
		wantVerdict: trace.VerdictFail,
		wantMessage: "every validation passed, but validationConfigurations.required failed: image ghcr.io/x/app:1.0 is not verified",
		wantImages:  true,
	}, {
		name:        "exempted by an exception",
		validation:  "false",
		request:     podRequest("Pod", "pods", "default"),
		exception:   true,
		wantRule:    engineapi.RuleStatusSkip,
		wantApplied: true,
		wantVerdict: trace.VerdictSkip,
		wantMessage: "rule is skipped due to policy exception: default/allow-web",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := tracePolicy(tt.validation)
			policy.Spec.ValidationConfigurations.Required = ptr.To(tt.required)
			var exceptions []*policiesv1beta1.PolicyException
			if tt.exception {
				exceptions = append(exceptions, &policiesv1beta1.PolicyException{
					ObjectMeta: metav1.ObjectMeta{Name: "allow-web", Namespace: "default"},
					Spec: policiesv1beta1.PolicyExceptionSpec{
						PolicyRefs:      []policiesv1beta1.PolicyRef{{Name: policy.Name, Kind: "ImageValidatingPolicy"}},
						MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "is-web", Expression: "object.metadata.name == 'web'"}},
					},
				})
				policy.Kind = "ImageValidatingPolicy"
			}

			untraced := handleTraced(t, false, policy, tt.request, exceptions...)
			traced := handleTraced(t, true, policy, tt.request, exceptions...)

			// tracing never changes what is reported
			if tt.wantRule == "" {
				assert.Empty(t, untraced, "tracing off: a policy that does not apply reports nothing")
				require.Len(t, traced, 1, "tracing on: it is kept for its trace")
				assert.Empty(t, traced[0].Result.Name(), "with no rule result")
			} else {
				require.Len(t, untraced, 1)
				require.Len(t, traced, 1)
				assert.Equal(t, tt.wantRule, untraced[0].Result.Status())
				assert.Equal(t, untraced[0].Result.Status(), traced[0].Result.Status())
				assert.Equal(t, untraced[0].Result.Message(), traced[0].Result.Message())
				assert.Nil(t, untraced[0].Trace, "no trace when compiled without tracing")
			}

			d := traced[0].Trace
			require.NotNil(t, d)
			assert.Equal(t, "trace-ivpol", d.PolicyName)
			assert.Equal(t, "ImageValidatingPolicy", d.PolicyKind)
			assert.Equal(t, "web", d.ResourceName)
			assert.Equal(t, "default", d.ResourceNamespace)
			assert.Equal(t, tt.wantApplied, d.Scope.Applied)
			assert.NotEmpty(t, d.Scope.Reason)
			assert.Equal(t, tt.wantVerdict, d.Verdict.Status)
			if tt.wantMessage != "" {
				assert.Contains(t, d.Verdict.Message, tt.wantMessage)
			}
			if tt.wantImages {
				require.NotNil(t, d.Images)
				assert.Equal(t, []trace.ImageTrace{
					{Category: "containers", Image: "ghcr.io/x/app:1.0", Checked: true},
					{Category: "containers", Image: "docker.io/library/busybox:1", Checked: false},
				}, d.Images.Found)
			}
		})
	}
}

func TestHandleValidating_TracingMatchConditionSkipIsNotAFailure(t *testing.T) {
	policy := tracePolicy("false")
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "only-prod", Expression: "object.metadata.namespace == 'prod'"}}

	assert.Empty(t, handleTraced(t, false, policy, podRequest("Pod", "pods", "default")), "tracing off: skipped, nothing reported")

	traced := handleTraced(t, true, policy, podRequest("Pod", "pods", "default"))
	require.Len(t, traced, 1)
	assert.Empty(t, traced[0].Result.Name(), "a skipped policy reports no rule, in particular not a failure")
	require.NotNil(t, traced[0].Trace)
	assert.Equal(t, trace.VerdictSkip, traced[0].Trace.Verdict.Status)
	assert.Contains(t, traced[0].Trace.Verdict.Message, `match condition "only-prod" did not pass`)
	assert.True(t, traced[0].Trace.Scope.Applied, "matchConstraints applied; a match condition skipped it")
}

// TestHandleValidating_TracingExtractionMode covers a custom workload (a JobSet) checked through
// its synthesized pod-template Pods: a template a match condition skips must stay a skip with its
// trace, not turn into a failure, and a failing template's trace names the template.
func TestHandleValidating_TracingExtractionMode(t *testing.T) {
	policy := buildExtractionModeRequestPolicy()
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
		Name: "not-skipped-image", Expression: "!object.spec.containers.exists(c, c.image.startsWith('skip/'))",
	}}
	jobSet := func(image string) engine.EngineRequest {
		raw, err := jobSetWithImage(image).MarshalJSON()
		require.NoError(t, err)
		gvr := metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"}
		return engine.EngineRequest{Request: v1.AdmissionRequest{
			Operation:       v1.Create,
			Kind:            metav1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
			Resource:        gvr,
			RequestResource: &gvr,
			Namespace:       "default",
			Name:            "latest-tag-jobset",
			Object:          apiruntime.RawExtension{Raw: raw},
		}}
	}

	t.Run("every template skipped", func(t *testing.T) {
		assert.Empty(t, handleTraced(t, false, policy, jobSet("skip/app:1.0")), "tracing off: skipped, nothing reported")
		traced := handleTraced(t, true, policy, jobSet("skip/app:1.0"))
		require.Len(t, traced, 1)
		assert.Empty(t, traced[0].Result.Name(), "a skipped template is not a failure")
		require.NotNil(t, traced[0].Trace)
		assert.Equal(t, trace.VerdictSkip, traced[0].Trace.Verdict.Status)
		assert.Contains(t, traced[0].Trace.Verdict.Message, `match condition "not-skipped-image" did not pass`)
	})

	t.Run("a failing template is named", func(t *testing.T) {
		untraced := handleTraced(t, false, policy, jobSet("bash:latest"))
		traced := handleTraced(t, true, policy, jobSet("bash:latest"))
		require.Len(t, untraced, 1)
		require.Len(t, traced, 1)
		assert.Equal(t, engineapi.RuleStatusFail, untraced[0].Result.Status())
		assert.Equal(t, untraced[0].Result.Message(), traced[0].Result.Message())
		require.NotNil(t, traced[0].Trace)
		assert.Equal(t, trace.VerdictFail, traced[0].Trace.Verdict.Status)
		assert.Contains(t, traced[0].Trace.Verdict.Message, "pod template at")
		assert.Equal(t, "JobSet", traced[0].Trace.ResourceKind)
	})
}
