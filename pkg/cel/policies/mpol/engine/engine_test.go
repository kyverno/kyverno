package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
)

func TestHandlePolicy(t *testing.T) {
	tests := []struct {
		name           string
		policy         Policy
		attr           admission.Attributes
		namespace      *corev1.Namespace
		matcher        matching.Matcher
		expectedStatus engineapi.RuleStatus
		expectError    bool
	}{
		{
			name: "policy matches and mutates successfully",
			policy: Policy{
				Policy: policiesv1alpha1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-policy",
					},
					Spec: policiesv1alpha1.MutatingPolicySpec{
						MatchConstraints: policiesv1alpha1.MatchConstraints{
							ResourceRules: []policiesv1alpha1.ResourceRule{
								{
									APIGroups:   []string{"apps"},
									APIVersions: []string{"v1"},
									Resources:   []string{"deployments"},
									Operations:  []string{"CREATE"},
								},
							},
						},
					},
				},
				CompiledPolicy: &mockCompiledPolicy{
					evaluateResult: &EvaluationResult{
						PatchedResource: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"metadata": map[string]interface{}{
									"labels": map[string]interface{}{
										"managed": "true",
									},
								},
							},
						},
					},
				},
			},
			attr: admission.NewAttributesRecord(
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "test-deployment",
						},
					},
				},
				nil,
				schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				"default",
				"test-deployment",
				schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				"",
				admission.Create,
				nil,
				false,
				nil,
			),
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
			matcher:        &mockMatcher{match: true},
			expectedStatus: engineapi.RuleStatusPass,
			expectError:    false,
		},
		{
			name: "policy does not match",
			policy: Policy{
				Policy: policiesv1alpha1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-policy",
					},
					Spec: policiesv1alpha1.MutatingPolicySpec{
						MatchConstraints: policiesv1alpha1.MatchConstraints{
							ResourceRules: []policiesv1alpha1.ResourceRule{
								{
									APIGroups:   []string{"apps"},
									APIVersions: []string{"v1"},
									Resources:   []string{"deployments"},
									Operations:  []string{"CREATE"},
								},
							},
						},
					},
				},
			},
			attr: admission.NewAttributesRecord(
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "test-deployment",
						},
					},
				},
				nil,
				schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				"default",
				"test-deployment",
				schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				"",
				admission.Create,
				nil,
				false,
				nil,
			),
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
			matcher:        &mockMatcher{match: false},
			expectedStatus: engineapi.RuleStatusSkip,
			expectError:    false,
		},
		{
			name: "policy evaluation fails",
			policy: Policy{
				Policy: policiesv1alpha1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-policy",
					},
					Spec: policiesv1alpha1.MutatingPolicySpec{
						MatchConstraints: policiesv1alpha1.MatchConstraints{
							ResourceRules: []policiesv1alpha1.ResourceRule{
								{
									APIGroups:   []string{"apps"},
									APIVersions: []string{"v1"},
									Resources:   []string{"deployments"},
									Operations:  []string{"CREATE"},
								},
							},
						},
					},
				},
				CompiledPolicy: &mockCompiledPolicy{
					evaluateResult: &EvaluationResult{
						Error: assert.AnError,
					},
				},
			},
			attr: admission.NewAttributesRecord(
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "test-deployment",
						},
					},
				},
				nil,
				schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				"default",
				"test-deployment",
				schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				"",
				admission.Create,
				nil,
				false,
				nil,
			),
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
			},
			matcher:        &mockMatcher{match: true},
			expectedStatus: engineapi.RuleStatusError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &engineImpl{
				matcher: tt.matcher,
			}

			typeConverter := patch.NewTypeConverterManager(nil, nil)
			response, patchedResource := engine.handlePolicy(context.Background(), tt.policy, tt.attr, tt.namespace, typeConverter)

			assert.Equal(t, tt.policy.Policy.Name, response.Policy.Name)
			if tt.expectError {
				assert.Equal(t, tt.expectedStatus, response.Rules[0].Status())
				assert.Contains(t, response.Rules[0].Message(), assert.AnError.Error())
			} else if tt.matcher.(*mockMatcher).match {
				if tt.policy.CompiledPolicy.(*mockCompiledPolicy).evaluateResult != nil {
					assert.Equal(t, tt.expectedStatus, response.Rules[0].Status())
					assert.NotNil(t, patchedResource)
				} else {
					assert.Equal(t, engineapi.RuleStatusSkip, response.Rules[0].Status())
				}
			} else {
				assert.Empty(t, response.Rules)
			}
		})
	}
}

// Mock implementations for testing
type mockMatcher struct {
	match bool
	err   error
}

func (m *mockMatcher) Match(criteria *matching.MatchCriteria, attr admission.Attributes, namespace *corev1.Namespace) (bool, error) {
	return m.match, m.err
}

type mockCompiledPolicy struct {
	evaluateResult *EvaluationResult
}

func (m *mockCompiledPolicy) Evaluate(ctx context.Context, attr admission.Attributes, namespace *corev1.Namespace, typeConverter patch.TypeConverterManager) *EvaluationResult {
	return m.evaluateResult
}
