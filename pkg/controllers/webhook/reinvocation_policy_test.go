package webhook

import (
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/client-go/tools/cache"
)

func TestBuildForJSONPoliciesMutationSetsReinvocationPolicy(t *testing.T) {
	mpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			ReinvocationPolicy: admissionregistrationv1.IfNeededReinvocationPolicy,
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
							},
						},
					},
				},
			},
		},
	}
	require.NoError(t, mpolIndexer.Add(policy))
	leaseIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, leaseIndexer.Add(&coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno-health",
			Namespace: config.KyvernoNamespace(),
			Annotations: map[string]string{
				AnnotationLastRequestTime: time.Now().Format(time.RFC3339),
			},
		},
	}))
	emptyIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c := &controller{
		mpolLister:         policiesv1beta1listers.NewMutatingPolicyLister(mpolIndexer),
		nmpolLister:        policiesv1beta1listers.NewNamespacedMutatingPolicyLister(emptyIndexer),
		ivpolLister:        policiesv1beta1listers.NewImageValidatingPolicyLister(emptyIndexer),
		nivpolLister:       policiesv1beta1listers.NewNamespacedImageValidatingPolicyLister(emptyIndexer),
		leaseLister:        coordinationv1listers.NewLeaseLister(leaseIndexer),
		stateRecorder:      NewStateRecorder(nil),
		celExpressionCache: NewExpressionCache(),
	}
	result := &admissionregistrationv1.MutatingWebhookConfiguration{}
	err := c.buildForJSONPoliciesMutation(
		config.NewDefaultConfiguration(false),
		nil,
		result,
	)
	require.NoError(t, err)
	var mpolWebhook *admissionregistrationv1.MutatingWebhook
	for i := range result.Webhooks {
		if result.Webhooks[i].Name == config.MutatingPolicyWebhookName+"-fail" {
			mpolWebhook = &result.Webhooks[i]
			break
		}
	}
	require.NotNil(t, mpolWebhook, "MutatingPolicy webhook should be generated")
	require.NotNil(t, mpolWebhook.ReinvocationPolicy, "MutatingPolicy webhook must set reinvocationPolicy")
	require.Equal(t, admissionregistrationv1.IfNeededReinvocationPolicy, *mpolWebhook.ReinvocationPolicy)
}
