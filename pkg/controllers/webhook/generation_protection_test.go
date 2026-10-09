package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/background/common"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	webhookaccessor "k8s.io/apiserver/pkg/admission/plugin/webhook"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/object"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

func TestGenerationLabelProtectionObjectSelector(t *testing.T) {
	t.Parallel()
	c := &controller{defaultTimeout: 10, servicePort: 443}
	webhook := c.buildGenerationLabelProtectionWebhook(nil)
	accessor := webhookaccessor.NewValidatingWebhookAccessor("protection", config.ValidatingWebhookConfigurationName, &webhook)
	matcher := &object.Matcher{}
	resource := func(content map[string]any) runtime.Object {
		return &unstructured.Unstructured{Object: content}
	}
	withLabels := func(labels any) runtime.Object {
		return resource(map[string]any{"metadata": map[string]any{"labels": labels}})
	}
	associated := withLabels(map[string]any{common.GeneratePolicyLabel: "policy"})
	tests := []struct {
		name        string
		new, old    runtime.Object
		wantMatched bool
	}{
		{name: "absent objects"},
		{name: "metadata-less subresource", new: resource(map[string]any{"spec": map[string]any{"replicas": int64(2)}})},
		{name: "null metadata", new: resource(map[string]any{"metadata": nil})},
		{name: "ordinary bootstrap without labels", new: resource(map[string]any{"kind": "Node", "metadata": map[string]any{"name": "node"}})},
		{name: "empty labels", new: withLabels(map[string]any{})},
		{name: "null labels", new: withLabels(nil)},
		{name: "ordinary labels", new: withLabels(map[string]any{"app": "example"})},
		{name: "standalone managed-by", new: withLabels(map[string]any{"app.kubernetes.io/managed-by": "kyverno"})},
		{name: "clone source marker", new: withLabels(map[string]any{common.GenerateTypeCloneSourceLabel: ""})},
		{name: "partial metadata has no policy association", new: withLabels(map[string]any{common.GenerateSourceUIDLabel: "source"})},
		{name: "create policy association", new: associated, wantMatched: true},
		{name: "empty policy association still matches", new: withLabels(map[string]any{common.GeneratePolicyLabel: ""}), wantMatched: true},
		{name: "unchanged policy association", new: associated, old: associated, wantMatched: true},
		{name: "remove all labels", new: resource(map[string]any{"metadata": map[string]any{}}), old: associated, wantMatched: true},
		{name: "remove policy association but retain other labels", new: withLabels(map[string]any{common.GenerateSourceUIDLabel: "source"}), old: associated, wantMatched: true},
		{name: "replace labels with null", new: withLabels(nil), old: associated, wantMatched: true},
		{name: "null new object", old: associated, wantMatched: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			operation := admission.Update
			if test.old == nil {
				operation = admission.Create
			}
			attributes := admission.NewAttributesRecord(test.new, test.old, schema.GroupVersionKind{}, "", "", schema.GroupVersionResource{}, "", operation, nil, false, nil)
			matched, err := matcher.MatchObjectSelector(accessor, attributes)
			require.Nil(t, err)
			assert.Equal(t, test.wantMatched, matched)
		})
	}
}

type generationProtectionConfiguration struct {
	config.Configuration
}

func (generationProtectionConfiguration) GetWebhook() config.WebhookConfig {
	return config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"scope": "triggers"}},
		ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"scope": "triggers"}},
	}
}

func TestGenerationLabelProtectionWithoutPolicies(t *testing.T) {
	t.Parallel()
	for _, automatic := range []bool{false, true} {
		t.Run(map[bool]string{false: "manual routing", true: "automatic routing"}[automatic], func(t *testing.T) {
			t.Parallel()
			factory := informers.NewSharedInformerFactory(versionedfake.NewSimpleClientset(), 0)
			leases := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, leases.Add(&coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
				Name: "kyverno-health", Namespace: config.KyvernoNamespace(),
				Annotations: map[string]string{AnnotationLastRequestTime: time.Now().Format(time.RFC3339)},
			}}))
			c := &controller{
				defaultTimeout: 10, servicePort: 443, excludeBootstrapResources: true,
				clusterroleLister:  rbacv1listers.NewClusterRoleLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
				leaseLister:        coordinationv1listers.NewLeaseLister(leases),
				cpolLister:         factory.Kyverno().V1().ClusterPolicies().Lister(),
				polLister:          factory.Kyverno().V1().Policies().Lister(),
				vpolLister:         factory.Policies().V1beta1().ValidatingPolicies().Lister(),
				nvpolLister:        factory.Policies().V1beta1().NamespacedValidatingPolicies().Lister(),
				gpolLister:         factory.Policies().V1beta1().GeneratingPolicies().Lister(),
				ngpolLister:        factory.Policies().V1beta1().NamespacedGeneratingPolicies().Lister(),
				ivpolLister:        factory.Policies().V1beta1().ImageValidatingPolicies().Lister(),
				nivpolLister:       factory.Policies().V1beta1().NamespacedImageValidatingPolicies().Lister(),
				celExpressionCache: NewExpressionCache(),
			}
			cfg := generationProtectionConfiguration{config.NewDefaultConfiguration(false)}
			build := c.buildDefaultResourceValidatingWebhookConfiguration
			if automatic {
				build = c.buildResourceValidatingWebhookConfiguration
			}
			result, err := build(context.Background(), cfg, []byte("ca"))
			require.NoError(t, err)
			var protection []admissionregistrationv1.ValidatingWebhook
			for _, webhook := range result.Webhooks {
				if webhook.Name == config.GenerationLabelProtectionWebhookName {
					protection = append(protection, webhook)
				}
			}
			require.Len(t, protection, 1)
			webhook := protection[0]
			assert.Nil(t, webhook.NamespaceSelector)
			assert.Equal(t, &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key: common.GeneratePolicyLabel, Operator: metav1.LabelSelectorOpExists,
			}}}, webhook.ObjectSelector)
			assert.Empty(t, webhook.MatchConditions, "metadata protection must work without the MatchConditions feature")
			assert.Equal(t, admissionregistrationv1.Fail, *webhook.FailurePolicy)
			assert.Equal(t, admissionregistrationv1.SideEffectClassNone, *webhook.SideEffects)
			assert.Equal(t, int32(10), *webhook.TimeoutSeconds)
			require.NotNil(t, webhook.ClientConfig.Service)
			assert.Equal(t, config.GenerationLabelProtectionWebhookServicePath, *webhook.ClientConfig.Service.Path)
			require.Len(t, webhook.Rules, 1)
			assert.Equal(t, []string{"*"}, webhook.Rules[0].APIGroups)
			assert.Equal(t, []string{"*"}, webhook.Rules[0].APIVersions)
			assert.Equal(t, []string{"*/*"}, webhook.Rules[0].Resources)
			assert.Equal(t, []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}, webhook.Rules[0].Operations)
		})
	}
}

func TestGenerationLabelProtectionManualRoutingUpgrade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	manual := admissionregistrationv1.ValidatingWebhook{Name: "user-managed.example.com", NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "custom"}}}
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "add protection", true: "refresh protection"}[existing], func(t *testing.T) {
			t.Parallel()
			observed := &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: config.ValidatingWebhookConfigurationName, Labels: map[string]string{"custom": "label"}, Annotations: map[string]string{"custom": "annotation"}},
				Webhooks:   []admissionregistrationv1.ValidatingWebhook{manual},
			}
			if existing {
				observed.Webhooks = append(observed.Webhooks, admissionregistrationv1.ValidatingWebhook{Name: config.GenerationLabelProtectionWebhookName})
			}
			client := kubefake.NewClientset(observed.DeepCopy())
			webhooks := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, webhooks.Add(observed))
			secrets := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, secrets.Add(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: config.KyvernoNamespace()}, Data: map[string][]byte{corev1.TLSCertKey: []byte("ca")}}))
			c := &controller{
				defaultTimeout: 10, servicePort: 443, caSecretName: "ca",
				configuration:     config.NewDefaultConfiguration(false),
				vwcClient:         client.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
				vwcLister:         admissionregistrationv1listers.NewValidatingWebhookConfigurationLister(webhooks),
				secretLister:      corev1listers.NewSecretLister(secrets),
				clusterroleLister: rbacv1listers.NewClusterRoleLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
			}
			require.NoError(t, c.reconcileResourceValidatingWebhookConfiguration(ctx))
			updated, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, observed.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, observed.Name, updated.Name)
			assert.Equal(t, observed.Labels, updated.Labels)
			assert.Equal(t, observed.Annotations, updated.Annotations)
			assert.Equal(t, observed.OwnerReferences, updated.OwnerReferences)
			require.Len(t, updated.Webhooks, 2)
			assert.Equal(t, manual, updated.Webhooks[0])
			assert.Equal(t, c.buildGenerationLabelProtectionWebhook([]byte("ca")), updated.Webhooks[1])
			require.NoError(t, webhooks.Update(updated))
			client.ClearActions()
			require.NoError(t, c.reconcileResourceValidatingWebhookConfiguration(ctx))
			assert.Empty(t, client.Actions(), "an unchanged reserved entry must not trigger another write")
		})
	}
}
