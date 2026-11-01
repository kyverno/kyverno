package webhook

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/cel/environment"
)

// matchWebhook evaluates a webhook's match conditions the way the API server does before it
// calls Kyverno (see GetCompiledMatcher in k8s.io/apiserver/pkg/admission/plugin/webhook).
func matchWebhook(t *testing.T, webhook admissionregistrationv1.ValidatingWebhook, attr admission.Attributes) matchconditions.MatchResult {
	t.Helper()
	expressions := make([]cel.ExpressionAccessor, 0, len(webhook.MatchConditions))
	for _, condition := range webhook.MatchConditions {
		expressions = append(expressions, &matchconditions.MatchCondition{Name: condition.Name, Expression: condition.Expression})
	}
	compiler := cel.NewConditionCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion()))
	evaluator := compiler.CompileCondition(expressions, cel.OptionalVariableDeclarations{HasAuthorizer: true}, environment.StoredExpressions)
	matcher := matchconditions.NewMatcher(evaluator, webhook.FailurePolicy, "webhook", "validating", webhook.Name)
	return matcher.Match(context.Background(), &admission.VersionedAttributes{
		Attributes:         attr,
		VersionedObject:    admission.NewLazyObject(attr.GetObject()),
		VersionedOldObject: admission.NewLazyObject(attr.GetOldObject()),
		VersionedKind:      attr.GetKind(),
	}, nil, nil)
}

// The autogen wrapping of a pod policy's match conditions must not turn a DELETE, where the API
// server binds object to null, into a match condition error: with failurePolicy Fail that rejects
// the DELETE before Kyverno is called, with Ignore it skips Kyverno for the DELETE altogether.
func TestBuildWebhookRules_PodPolicyMatchConditionsOnDelete(t *testing.T) {
	podGVK := schema.GroupVersionKind{Version: "v1", Kind: "Pod"}
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	pod := func(labels map[string]string) runtime.Object {
		return &corev1.Pod{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
			ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "default", Labels: labels},
		}
	}
	deleteAttr := func(old runtime.Object) admission.Attributes {
		return admission.NewAttributesRecord(nil, old, podGVK, "default", "web-1", podGVR, "", admission.Delete, &metav1.DeleteOptions{}, false, &user.DefaultInfo{Name: "alice"})
	}
	createAttr := func(obj runtime.Object) admission.Attributes {
		return admission.NewAttributesRecord(obj, nil, podGVK, "default", "web-1", podGVR, "", admission.Create, &metav1.CreateOptions{}, false, &user.DefaultInfo{Name: "alice"})
	}
	newPolicy := func(failurePolicy admissionregistrationv1.FailurePolicyType, condition string, operations ...admissionregistrationv1.OperationType) *policiesv1beta1.ValidatingPolicy {
		return &policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "protect-pods"},
			Spec: policiesv1beta1.ValidatingPolicySpec{
				FailurePolicy: &failurePolicy,
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: operations,
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					}},
				},
				MatchConditions: []admissionregistrationv1.MatchCondition{{
					Name:       "condition",
					Expression: condition,
				}},
				Validations: []admissionregistrationv1.Validation{{Expression: "false", Message: "denied"}},
			},
		}
	}
	const protectedOnly = "has(oldObject.metadata.labels) && 'protected' in oldObject.metadata.labels"
	const labelledOnly = "has(object.metadata.labels) && 'team' in object.metadata.labels"
	tests := []struct {
		name          string
		failurePolicy admissionregistrationv1.FailurePolicyType
		condition     string
		operations    []admissionregistrationv1.OperationType
		attr          admission.Attributes
		wantMatch     bool
	}{{
		name:          "delete of a pod the condition excludes skips the webhook",
		failurePolicy: admissionregistrationv1.Fail,
		condition:     protectedOnly,
		operations:    []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
		attr:          deleteAttr(pod(nil)),
	}, {
		name:          "delete of a pod the condition selects calls the webhook",
		failurePolicy: admissionregistrationv1.Fail,
		condition:     protectedOnly,
		operations:    []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
		attr:          deleteAttr(pod(map[string]string{"protected": "true"})),
		wantMatch:     true,
	}, {
		name:          "delete of a pod the condition selects calls the webhook with failurePolicy Ignore",
		failurePolicy: admissionregistrationv1.Ignore,
		condition:     protectedOnly,
		operations:    []admissionregistrationv1.OperationType{admissionregistrationv1.Delete},
		attr:          deleteAttr(pod(map[string]string{"protected": "true"})),
		wantMatch:     true,
	}, {
		name:          "create of a pod the condition selects calls the webhook",
		failurePolicy: admissionregistrationv1.Fail,
		condition:     labelledOnly,
		operations:    []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
		attr:          createAttr(pod(map[string]string{"team": "a"})),
		wantMatch:     true,
	}, {
		name:          "create of a pod the condition excludes skips the webhook",
		failurePolicy: admissionregistrationv1.Fail,
		condition:     labelledOnly,
		operations:    []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
		attr:          createAttr(pod(nil)),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vpol := newPolicy(tt.failurePolicy, tt.condition, tt.operations...)
			expressionCache := NewExpressionCache()
			expressionCache.AddPolicyExpressions(vpol.GetMatchConditions())
			webhooks := buildWebhookRules(
				config.NewDefaultConfiguration(false),
				"",
				config.ValidatingPolicyWebhookName,
				"/vpol",
				0,
				nil,
				[]engineapi.GenericPolicy{engineapi.NewValidatingPolicy(vpol)},
				expressionCache,
			)
			require.Len(t, webhooks, 1)
			require.NotEmpty(t, webhooks[0].MatchConditions, "the policy's match condition should be offloaded to the webhook")

			result := matchWebhook(t, webhooks[0], tt.attr)

			assert.NoError(t, result.Error, "the API server must not reject the request on a match condition error")
			assert.Equal(t, tt.wantMatch, result.Matches, "failed condition: %q", result.FailedConditionName)
		})
	}
}
