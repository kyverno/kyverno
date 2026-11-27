package webhook

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

// webhookDefaultFixtures builds desired and API-defaulted observed webhooks
// independently of production helpers; the fake client does not apply defaults.
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
	// A single selector can pass through mergeLabelSelectors unchanged. Empty
	// maps and slices are omitted by JSON serialization on the API server.
	emptyFields := *minimal.DeepCopy()
	emptyFields.Name = "empty-selector-fields.kyverno.svc"
	emptyFields.NamespaceSelector = &metav1.LabelSelector{
		MatchLabels:      map[string]string{},
		MatchExpressions: []metav1.LabelSelectorRequirement{},
	}
	emptyFields.ObjectSelector = &metav1.LabelSelector{
		MatchLabels:      map[string]string{"app": "test"},
		MatchExpressions: []metav1.LabelSelectorRequirement{},
	}
	storedEmptyFields := *stored.DeepCopy()
	storedEmptyFields.Name = emptyFields.Name
	storedEmptyFields.NamespaceSelector = &metav1.LabelSelector{}
	storedEmptyFields.ObjectSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}}
	// Exists and DoesNotExist take no values, and an empty list is omitted the same way.
	emptyValues := *minimal.DeepCopy()
	emptyValues.Name = "empty-requirement-values.kyverno.svc"
	emptyValues.NamespaceSelector = &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: "team", Operator: metav1.LabelSelectorOpExists, Values: []string{}},
	}}
	emptyValues.ObjectSelector = &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"test"}},
		{Key: "skip", Operator: metav1.LabelSelectorOpDoesNotExist, Values: []string{}},
	}}
	storedEmptyValues := *stored.DeepCopy()
	storedEmptyValues.Name = emptyValues.Name
	storedEmptyValues.NamespaceSelector = &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: "team", Operator: metav1.LabelSelectorOpExists},
	}}
	storedEmptyValues.ObjectSelector = &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"test"}},
		{Key: "skip", Operator: metav1.LabelSelectorOpDoesNotExist},
	}}
	desired, observed := []admissionregistrationv1.ValidatingWebhook{minimal, explicit, url, emptyFields, emptyValues}, []admissionregistrationv1.ValidatingWebhook{stored, explicit, storedURL, storedEmptyFields, storedEmptyValues}

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

// mutatingDefaultFixture converts the shared fixtures and sets the API-server
// reinvocation default only on the stored variant.
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

// TestDefaultWebhookSelectorDoesNotMutateSource checks that normalization does
// not change a selector shared with a policy or configuration.
func TestDefaultWebhookSelectorDoesNotMutateSource(t *testing.T) {
	t.Parallel()
	selector := &metav1.LabelSelector{
		MatchLabels:      map[string]string{},
		MatchExpressions: []metav1.LabelSelectorRequirement{},
	}
	requirements := &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
		{Key: "team", Operator: metav1.LabelSelectorOpExists, Values: []string{}},
	}}
	webhooks := []admissionregistrationv1.ValidatingWebhook{{NamespaceSelector: selector, ObjectSelector: requirements}}
	defaultValidatingWebhooks(webhooks)
	require.Equal(t, &metav1.LabelSelector{}, webhooks[0].NamespaceSelector)
	require.Nil(t, webhooks[0].ObjectSelector.MatchExpressions[0].Values)
	require.NotNil(t, selector.MatchLabels)
	require.NotNil(t, selector.MatchExpressions)
	require.NotNil(t, requirements.MatchExpressions[0].Values)
}

// TestReconcileWebhookConfigurationDefaults verifies that defaulted webhooks
// avoid repeated writes while creation and real changes still persist.
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

// newTestIndexer returns an indexer holding objs.
func newTestIndexer(t *testing.T, objs ...any) cache.Indexer {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, obj := range objs {
		require.NoError(t, indexer.Add(obj))
	}
	return indexer
}

// newResourceWebhookTestController returns a controller that can run the complete
// resource webhook builders with the given ValidatingPolicies and MutatingPolicies.
func newResourceWebhookTestController(t *testing.T, client *fake.Clientset, webhooks cache.Indexer, vpols, mpols []any) *controller {
	t.Helper()
	return &controller{
		autoUpdateWebhooks: true,
		configuration:      config.NewDefaultConfiguration(false),
		caSecretName:       "root-ca",
		secretLister: corev1listers.NewSecretLister(newTestIndexer(t, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "root-ca", Namespace: config.KyvernoNamespace()},
			Data:       map[string][]byte{corev1.TLSCertKey: []byte("ca")},
		})),
		leaseLister: coordinationv1listers.NewLeaseLister(newTestIndexer(t, &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
			Name: "kyverno-health", Namespace: config.KyvernoNamespace(),
			Annotations: map[string]string{AnnotationLastRequestTime: time.Now().Format(time.RFC3339)},
		}})),
		clusterroleLister:  rbacv1listers.NewClusterRoleLister(newTestIndexer(t)),
		vpolLister:         policiesv1beta1listers.NewValidatingPolicyLister(newTestIndexer(t, vpols...)),
		mpolLister:         policiesv1beta1listers.NewMutatingPolicyLister(newTestIndexer(t, mpols...)),
		nvpolLister:        policiesv1beta1listers.NewNamespacedValidatingPolicyLister(newTestIndexer(t)),
		gpolLister:         policiesv1beta1listers.NewGeneratingPolicyLister(newTestIndexer(t)),
		ngpolLister:        policiesv1beta1listers.NewNamespacedGeneratingPolicyLister(newTestIndexer(t)),
		ivpolLister:        policiesv1beta1listers.NewImageValidatingPolicyLister(newTestIndexer(t)),
		nivpolLister:       policiesv1beta1listers.NewNamespacedImageValidatingPolicyLister(newTestIndexer(t)),
		nmpolLister:        policiesv1beta1listers.NewNamespacedMutatingPolicyLister(newTestIndexer(t)),
		vwcLister:          admissionregistrationv1listers.NewValidatingWebhookConfigurationLister(webhooks),
		mwcLister:          admissionregistrationv1listers.NewMutatingWebhookConfigurationLister(webhooks),
		vwcClient:          client.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		mwcClient:          client.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		stateRecorder:      NewStateRecorder(nil),
		celExpressionCache: NewExpressionCache(),
	}
}

// withoutGenerationLabelProtection keeps the default toggles but disables the
// generation label protection webhook.
type withoutGenerationLabelProtection struct {
	toggle.Toggles
}

func (withoutGenerationLabelProtection) ProtectGenerationLabels() bool { return false }

// TestReconcileResourceWebhooksWithoutPolicies runs the complete resource builders
// without any policy. The mutating configuration then has no webhooks, and so has
// the validating one when generation label protection is disabled. The API server
// stores that empty list as an omitted field, so later watchdog ticks must compare
// it as equal instead of updating the configuration every time.
func TestReconcileResourceWebhooksWithoutPolicies(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name          string
		mutating      bool
		protectLabels bool
		wantWebhooks  bool
	}{
		{name: "validating", protectLabels: true, wantWebhooks: true},
		{name: "validating without generation label protection"},
		{name: "mutating", mutating: true, protectLabels: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			if !tt.protectLabels {
				ctx = toggle.NewContext(ctx, withoutGenerationLabelProtection{toggle.FromContext(ctx)})
			}
			client := fake.NewSimpleClientset()
			webhooks := newTestIndexer(t)
			c := newResourceWebhookTestController(t, client, webhooks, nil, nil)
			reconcile := c.reconcileResourceValidatingWebhookConfiguration
			var stored runtime.Object = &admissionregistrationv1.ValidatingWebhookConfiguration{}
			if tt.mutating {
				reconcile = c.reconcileResourceMutatingWebhookConfiguration
				stored = &admissionregistrationv1.MutatingWebhookConfiguration{}
			}
			require.NoError(t, reconcile(ctx))
			actions := client.Actions()
			require.Len(t, actions, 1)
			require.Equal(t, "create", actions[0].GetVerb())

			// Store the write as the API server returns it, after its JSON round trip.
			data, err := json.Marshal(actions[0].(clienttesting.CreateAction).GetObject())
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, stored))
			var count int
			switch obj := stored.(type) {
			case *admissionregistrationv1.ValidatingWebhookConfiguration:
				require.Equal(t, !tt.wantWebhooks, obj.Webhooks == nil)
				count = len(obj.Webhooks)
			case *admissionregistrationv1.MutatingWebhookConfiguration:
				require.Equal(t, !tt.wantWebhooks, obj.Webhooks == nil)
				count = len(obj.Webhooks)
			}
			require.Equal(t, tt.wantWebhooks, count > 0)
			require.NoError(t, webhooks.Add(stored))
			client.ClearActions()
			for range 3 {
				require.NoError(t, reconcile(ctx))
			}
			require.Empty(t, client.Actions(), "subsequent watchdog ticks must not send updates")
		})
	}
}

// TestReconcileResourceWebhooksWithOmittedRuleScope runs the complete resource
// builders for CEL policies whose otherwise identical rules differ only in an
// omitted versus explicit scope. Both must reconcile without panicking, collapse
// into one defaulted rule, and stay unchanged on later watchdog ticks.
func TestReconcileResourceWebhooksWithOmittedRuleScope(t *testing.T) {
	t.Parallel()
	rule := func(scope *admissionregistrationv1.ScopeType) *admissionregistrationv1.MatchResources {
		return &admissionregistrationv1.MatchResources{ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule: admissionregistrationv1.Rule{
					APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}, Scope: scope,
				},
			},
		}}}
	}
	vpols := []any{
		&policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "omitted-scope"}, Spec: policiesv1beta1.ValidatingPolicySpec{MatchConstraints: rule(nil)}},
		&policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "explicit-scope"}, Spec: policiesv1beta1.ValidatingPolicySpec{MatchConstraints: rule(ptr.To(admissionregistrationv1.AllScopes))}},
	}
	mpols := []any{
		&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "omitted-scope"}, Spec: policiesv1beta1.MutatingPolicySpec{MatchConstraints: rule(nil)}},
		&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "explicit-scope"}, Spec: policiesv1beta1.MutatingPolicySpec{MatchConstraints: rule(ptr.To(admissionregistrationv1.AllScopes))}},
	}
	for _, kind := range []string{"validating", "mutating"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			client := fake.NewSimpleClientset()
			webhooks := newTestIndexer(t)
			c := newResourceWebhookTestController(t, client, webhooks, vpols, mpols)
			reconcile := c.reconcileResourceValidatingWebhookConfiguration
			if kind == "mutating" {
				reconcile = c.reconcileResourceMutatingWebhookConfiguration
			}
			require.NotPanics(t, func() { require.NoError(t, reconcile(t.Context())) })
			actions := client.Actions()
			require.Len(t, actions, 1)
			require.Equal(t, "create", actions[0].GetVerb())
			written := actions[0].(clienttesting.CreateAction).GetObject()
			var rules []admissionregistrationv1.RuleWithOperations
			switch obj := written.(type) {
			case *admissionregistrationv1.ValidatingWebhookConfiguration:
				// The generation label protection webhook is always added; check the policy webhook.
				var policyWebhooks []admissionregistrationv1.ValidatingWebhook
				for _, w := range obj.Webhooks {
					if w.Name != config.GenerationLabelProtectionWebhookName {
						policyWebhooks = append(policyWebhooks, w)
					}
				}
				require.Len(t, policyWebhooks, 1)
				rules = policyWebhooks[0].Rules
			case *admissionregistrationv1.MutatingWebhookConfiguration:
				require.Len(t, obj.Webhooks, 1)
				rules = obj.Webhooks[0].Rules
			}
			require.Len(t, rules, 1, "omitted and explicit default scopes must deduplicate")
			require.Equal(t, ptr.To(admissionregistrationv1.AllScopes), rules[0].Scope)

			// Simulate the informer observing the write before the next tick.
			require.NoError(t, webhooks.Add(written.DeepCopyObject()))
			client.ClearActions()
			for range 3 {
				require.NoError(t, reconcile(t.Context()))
			}
			require.Empty(t, client.Actions(), "subsequent watchdog ticks must not send updates")
		})
	}
}

// TestReconcileGenerationLabelProtectionWithoutAutoUpdate covers the one webhook
// Kyverno still manages in the resource validating configuration when
// autoUpdateWebhooks is disabled: it is added once with the API-server defaults,
// and later watchdog ticks leave the configuration alone.
func TestReconcileGenerationLabelProtectionWithoutAutoUpdate(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"present", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			c := &controller{servicePort: 443, defaultTimeout: 10, caSecretName: "root-ca"}
			// The builder leaves the namespace selector and the rule scope for the API server to fill in.
			stored := c.buildGenerationLabelProtectionWebhook(t.Context(), []byte("ca"))
			stored.NamespaceSelector = &metav1.LabelSelector{}
			stored.Rules[0].Scope = ptr.To(admissionregistrationv1.AllScopes)
			// Policy routing is user-managed in this mode and must be kept as is.
			_, userManaged := webhookDefaultFixtures(t)
			observed := &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: config.ValidatingWebhookConfigurationName},
				Webhooks:   userManaged[:1],
			}
			if scenario == "present" {
				observed.Webhooks = append(observed.Webhooks, *stored.DeepCopy())
			}
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, indexer.Add(observed))
			client := fake.NewSimpleClientset(observed.DeepCopy())
			secrets := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, secrets.Add(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "root-ca", Namespace: config.KyvernoNamespace()},
				Data:       map[string][]byte{corev1.TLSCertKey: []byte("ca")},
			}))
			c.secretLister = corev1listers.NewSecretLister(secrets)
			c.vwcLister = admissionregistrationv1listers.NewValidatingWebhookConfigurationLister(indexer)
			c.vwcClient = client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
			reconcile := func() error {
				return c.reconcileValidatingWebhookConfiguration(t.Context(), false, func(context.Context, config.Configuration, []byte) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
					return &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: config.ValidatingWebhookConfigurationName}}, nil
				})
			}
			require.NoError(t, reconcile())
			actions := client.Actions()
			if scenario == "missing" {
				require.Len(t, actions, 1)
				require.Equal(t, "update", actions[0].GetVerb())
				written := actions[0].(clienttesting.UpdateAction).GetObject().(*admissionregistrationv1.ValidatingWebhookConfiguration)
				require.Equal(t, []admissionregistrationv1.ValidatingWebhook{userManaged[0], stored}, written.Webhooks)
				// Simulate the informer observing the write before the next tick.
				require.NoError(t, indexer.Update(written.DeepCopy()))
			} else {
				require.Empty(t, actions, "an unchanged protection webhook must not be updated")
			}
			client.ClearActions()
			for range 3 {
				require.NoError(t, reconcile())
			}
			require.Empty(t, client.Actions(), "subsequent watchdog ticks must not send updates")
		})
	}
}
