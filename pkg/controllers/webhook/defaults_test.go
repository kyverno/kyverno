package webhook

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

// Use explicit API-server defaults for the observed fixtures, independently of
// the production defaulting helpers. The fake client does not apply defaults.
func webhookDefaultFixtures(t *testing.T) ([]admissionregistrationv1.ValidatingWebhook, []admissionregistrationv1.ValidatingWebhook) {
	t.Helper()
	rules := []admissionregistrationv1.RuleWithOperations{{
		Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		Rule: admissionregistrationv1.Rule{
			APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"},
		},
	}}
	minimal := admissionregistrationv1.ValidatingWebhook{
		Name: "minimal.kyverno.svc",
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{Namespace: "kyverno", Name: "kyverno-svc"},
		},
		Rules: rules, SideEffects: ptr.To(admissionregistrationv1.SideEffectClassNone),
		AdmissionReviewVersions: []string{"v1"},
	}
	stored := *minimal.DeepCopy()
	stored.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
	stored.MatchPolicy = ptr.To(admissionregistrationv1.Equivalent)
	stored.NamespaceSelector = &metav1.LabelSelector{}
	stored.ObjectSelector = &metav1.LabelSelector{}
	stored.TimeoutSeconds = ptr.To[int32](10)
	stored.Rules[0].Scope = ptr.To(admissionregistrationv1.AllScopes)
	stored.ClientConfig.Service.Port = ptr.To[int32](443)

	// Explicit settings must survive normalization, including selectors, scope,
	// non-default timeouts and ports, and URL-based clients without a Service.
	explicit := *stored.DeepCopy()
	explicit.Name = "explicit.kyverno.svc"
	explicit.FailurePolicy = ptr.To(admissionregistrationv1.Ignore)
	explicit.MatchPolicy = ptr.To(admissionregistrationv1.Exact)
	explicit.NamespaceSelector.MatchLabels = map[string]string{"team": "platform"}
	explicit.ObjectSelector.MatchLabels = map[string]string{"app": "test"}
	explicit.TimeoutSeconds = ptr.To[int32](25)
	explicit.Rules[0].Scope = ptr.To(admissionregistrationv1.NamespacedScope)
	explicit.ClientConfig.Service.Port = ptr.To[int32](9443)
	url := *minimal.DeepCopy()
	url.Name = "url.kyverno.svc"
	url.ClientConfig = admissionregistrationv1.WebhookClientConfig{URL: ptr.To("https://example.com/webhook")}
	storedURL := *stored.DeepCopy()
	storedURL.Name = url.Name
	storedURL.ClientConfig = url.ClientConfig
	desired, observed := []admissionregistrationv1.ValidatingWebhook{minimal, explicit, url}, []admissionregistrationv1.ValidatingWebhook{stored, explicit, storedURL}

	// Exercise the real CEL builder with the policy kinds reported in #17735.
	match := &admissionregistrationv1.MatchResources{ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{RuleWithOperations: rules[0]}}}
	policies := []engineapi.GenericPolicy{
		engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "disallow-latest-tag"},
			Spec:       policiesv1beta1.ValidatingPolicySpec{FailurePolicy: ptr.To(admissionregistrationv1.Ignore), MatchConstraints: match},
		}),
		engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "verify-images"},
			Spec:       policiesv1beta1.ImageValidatingPolicySpec{FailurePolicy: ptr.To(admissionregistrationv1.Ignore), MatchConstraints: match},
		}),
	}
	for _, policy := range policies {
		webhooks := buildWebhookRules(config.NewDefaultConfiguration(false), "", policy.GetName()+".kyverno.svc", "/policy", 443, []byte("ca"), []engineapi.GenericPolicy{policy}, NewExpressionCache())
		require.NotEmpty(t, webhooks)
		for _, w := range webhooks {
			desired = append(desired, w)
			s := *w.DeepCopy()
			s.MatchPolicy = ptr.To(admissionregistrationv1.Equivalent)
			s.TimeoutSeconds = ptr.To[int32](10)
			if s.NamespaceSelector == nil {
				s.NamespaceSelector = &metav1.LabelSelector{}
			}
			s.ObjectSelector = &metav1.LabelSelector{}
			for i := range s.Rules {
				s.Rules[i].Scope = ptr.To(admissionregistrationv1.AllScopes)
			}
			observed = append(observed, s)
		}
	}
	return desired, observed
}

func mutatingDefaultFixture(webhooks []admissionregistrationv1.ValidatingWebhook, stored bool) []admissionregistrationv1.MutatingWebhook {
	result := make([]admissionregistrationv1.MutatingWebhook, 0, len(webhooks))
	for i, w := range webhooks {
		m := admissionregistrationv1.MutatingWebhook{
			Name: w.Name, ClientConfig: w.ClientConfig, Rules: w.Rules,
			FailurePolicy: w.FailurePolicy, MatchPolicy: w.MatchPolicy,
			NamespaceSelector: w.NamespaceSelector, ObjectSelector: w.ObjectSelector,
			SideEffects: w.SideEffects, TimeoutSeconds: w.TimeoutSeconds,
			AdmissionReviewVersions: w.AdmissionReviewVersions,
		}
		if stored {
			m.ReinvocationPolicy = ptr.To(admissionregistrationv1.NeverReinvocationPolicy)
		}
		if i == 1 { // The explicit-settings fixture.
			m.ReinvocationPolicy = ptr.To(admissionregistrationv1.IfNeededReinvocationPolicy)
		}
		result = append(result, m)
	}
	return result
}

func TestReconcileWebhookConfigurationDefaults(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"validating", "mutating"} {
		for _, scenario := range []string{"unchanged", "changed", "create", "updates disabled"} {
			t.Run(kind+"/"+scenario, func(t *testing.T) {
				t.Parallel()
				desiredHooks, storedHooks := webhookDefaultFixtures(t)
				meta := metav1.ObjectMeta{Name: "test-webhooks", Labels: map[string]string{"app": "kyverno"}}
				var desired, expected runtime.Object
				if kind == "validating" {
					desired = &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: meta, Webhooks: desiredHooks}
					expected = &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: meta, Webhooks: storedHooks}
				} else {
					desired = &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: meta, Webhooks: mutatingDefaultFixture(desiredHooks, false)}
					expected = &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: meta, Webhooks: mutatingDefaultFixture(storedHooks, true)}
				}
				observed := expected.DeepCopyObject()
				if scenario == "changed" || scenario == "updates disabled" {
					switch obj := observed.(type) {
					case *admissionregistrationv1.ValidatingWebhookConfiguration:
						obj.Webhooks[0].TimeoutSeconds = ptr.To[int32](20)
						obj.Labels = map[string]string{"app": "old"}
					case *admissionregistrationv1.MutatingWebhookConfiguration:
						obj.Webhooks[0].TimeoutSeconds = ptr.To[int32](20)
						obj.Labels = map[string]string{"app": "old"}
					}
				}
				original := observed.DeepCopyObject()
				indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				client := fake.NewSimpleClientset()
				if scenario != "create" {
					require.NoError(t, indexer.Add(observed))
					require.NoError(t, client.Tracker().Add(observed.DeepCopyObject()))
				}
				secrets := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				require.NoError(t, secrets.Add(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "root-ca", Namespace: config.KyvernoNamespace()},
					Data:       map[string][]byte{corev1.TLSCertKey: []byte("ca")},
				}))
				c := &controller{
					caSecretName: "root-ca", secretLister: corev1listers.NewSecretLister(secrets),
					vwcLister: admissionregistrationv1listers.NewValidatingWebhookConfigurationLister(indexer),
					mwcLister: admissionregistrationv1listers.NewMutatingWebhookConfigurationLister(indexer),
					vwcClient: client.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
					mwcClient: client.AdmissionregistrationV1().MutatingWebhookConfigurations(),
				}
				reconcile := func() error {
					if obj, ok := desired.(*admissionregistrationv1.ValidatingWebhookConfiguration); ok {
						return c.reconcileValidatingWebhookConfiguration(t.Context(), scenario != "updates disabled", func(context.Context, config.Configuration, []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
							return obj.DeepCopy(), nil
						})
					}
					return c.reconcileMutatingWebhookConfiguration(t.Context(), scenario != "updates disabled", func(context.Context, config.Configuration, []byte) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
						return desired.(*admissionregistrationv1.MutatingWebhookConfiguration).DeepCopy(), nil
					})
				}
				require.NoError(t, reconcile())
				require.Equal(t, original, observed, "reconciliation must not mutate informer objects")
				actions := client.Actions()
				switch scenario {
				case "changed", "create":
					require.Len(t, actions, 1)
					var written runtime.Object
					if scenario == "create" {
						require.Equal(t, "create", actions[0].GetVerb())
						written = actions[0].(clienttesting.CreateAction).GetObject()
					} else {
						require.Equal(t, "update", actions[0].GetVerb())
						written = actions[0].(clienttesting.UpdateAction).GetObject()
					}
					require.Equal(t, expected, written)
					// Simulate the informer observing the write before the next tick.
					require.NoError(t, indexer.Update(written.DeepCopyObject()))
				default:
					require.Empty(t, actions, "an unchanged or unmanaged configuration must not be updated")
				}
				client.ClearActions()
				for range 3 {
					require.NoError(t, reconcile())
				}
				require.Empty(t, client.Actions(), "subsequent watchdog ticks must not send updates")
			})
		}
	}
}
