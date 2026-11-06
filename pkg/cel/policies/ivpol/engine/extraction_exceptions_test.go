package engine

import (
	"context"
	"fmt"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// jobSetWithImages builds a JobSet with one replicatedJob (so one pod template) per image, in order.
func jobSetWithImages(images ...string) *unstructured.Unstructured {
	jobs := make([]any, 0, len(images))
	for i, image := range images {
		jobs = append(jobs, map[string]any{
			"name": fmt.Sprintf("job-%d", i),
			"template": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
				"containers": []any{map[string]any{"name": "worker", "image": image}},
			}}}},
		})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "multi-template-jobset", "namespace": "default"},
		"spec":       map[string]any{"replicatedJobs": jobs},
	}}
}

// TestHandleValidating_ExtractionMode_ExemptTemplateNextToPassingTemplate: an exception that
// exempts one pod template must still be reported when another template passes, the same as when
// the exempted template is the only one. A failing template still wins, and required still applies
// to the images of the templates the exception did not cover.
func TestHandleValidating_ExtractionMode_ExemptTemplateNextToPassingTemplate(t *testing.T) {
	tests := []struct {
		name       string
		images     []string
		required   bool
		wantStatus engineapi.RuleStatus
		wantMsg    string
		wantExcs   int
	}{{
		name:       "exempted template only",
		images:     []string{"legacy/app:latest"},
		wantStatus: engineapi.RuleStatusSkip,
		wantMsg:    "rule is skipped due to policy exception: default/polex",
		wantExcs:   1,
	}, {
		name:       "exempted template before a passing one",
		images:     []string{"legacy/app:latest", "bash:1.0"},
		wantStatus: engineapi.RuleStatusSkip,
		wantMsg:    "rule is skipped due to policy exception: default/polex",
		wantExcs:   1,
	}, {
		name:       "exempted template after a passing one",
		images:     []string{"bash:1.0", "legacy/app:latest"},
		wantStatus: engineapi.RuleStatusSkip,
		wantMsg:    "rule is skipped due to policy exception: default/polex",
		wantExcs:   1,
	}, {
		name:       "a failing template is not hidden by an exempted one",
		images:     []string{"legacy/app:latest", "nginx:latest"},
		wantStatus: engineapi.RuleStatusFail,
		wantMsg:    "no latest tag (pod template at spec.replicatedJobs[1].template.spec.template)",
	}, {
		name:       "required still applies to a passing template before the exempted one",
		images:     []string{"bash:1.0", "legacy/app:latest"},
		required:   true,
		wantStatus: engineapi.RuleStatusFail,
		wantMsg:    "image bash:1.0 is not verified: no policy performed a signature or attestation check on it",
	}, {
		name:       "required still applies to a passing template after the exempted one",
		images:     []string{"legacy/app:latest", "bash:1.0"},
		required:   true,
		wantStatus: engineapi.RuleStatusFail,
		wantMsg:    "image bash:1.0 is not verified: no policy performed a signature or attestation check on it",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.ImageValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "disallow-latest-tag"},
				Spec: policiesv1beta1.ImageValidatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						}},
					},
					EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
						Mode: policieskyvernoio.EvaluationModeKubernetes,
					},
					ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
						MutateDigest: ptr.To(false),
						VerifyDigest: ptr.To(false),
						Required:     ptr.To(tt.required),
					},
					AutogenConfiguration: &policiesv1beta1.ImageValidatingPolicyAutogenConfiguration{
						PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
							Controllers: []string{"jobsets.v1alpha2.jobset.x-k8s.io"},
						},
					},
					MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "*"}},
					Validations: []admissionregistrationv1.Validation{{
						Expression: "object.spec.containers.all(c, !c.image.endsWith(':latest'))",
						Message:    "no latest tag",
					}},
				},
			}
			polex := &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "polex"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{Name: policy.GetName(), Kind: "ImageValidatingPolicy"}},
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "legacy",
						Expression: "object.spec.containers.exists(c, c.image.startsWith('legacy/'))",
					}},
				},
			}
			provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, []*policiesv1beta1.PolicyException{polex})
			require.NoError(t, err)
			eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false)).(*engineImpl)
			eng.newImageContext = func() (imagedataloader.ImageContext, error) {
				return fakeImageContext{}, nil
			}
			raw, err := jobSetWithImages(tt.images...).MarshalJSON()
			require.NoError(t, err)
			req := engine.EngineRequest{
				Request: v1.AdmissionRequest{
					Operation:       v1.Create,
					Kind:            metav1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
					Resource:        metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
					RequestResource: &metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
					Namespace:       "default",
					Name:            "multi-template-jobset",
					Object:          apiruntime.RawExtension{Raw: raw},
				},
			}
			resp, err := eng.HandleValidating(context.Background(), req, nil)
			require.NoError(t, err)
			require.Len(t, resp.Policies, 1)
			rule := resp.Policies[0].Result
			assert.Equal(t, tt.wantStatus, rule.Status())
			assert.Equal(t, tt.wantMsg, rule.Message())
			assert.Len(t, rule.Exceptions(), tt.wantExcs)
		})
	}
}
