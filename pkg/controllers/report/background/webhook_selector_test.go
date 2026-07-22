package background

import (
	"testing"

	policiesv1beta1api "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// fakeNamespaceLister satisfies corev1listers.NamespaceLister using a simple map.
type fakeNamespaceLister struct {
	namespaces map[string]*corev1.Namespace
}

func (f *fakeNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	if ns, ok := f.namespaces[name]; ok {
		return ns, nil
	}
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	return corev1listers.NewNamespaceLister(indexer).Get(name)
}

func (f *fakeNamespaceLister) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	var out []*corev1.Namespace
	for _, ns := range f.namespaces {
		if selector.Matches(labels.Set(ns.Labels)) {
			out = append(out, ns)
		}
	}
	return out, nil
}

// fakeConfig implements the subset of config.Configuration used by the helpers.
type fakeConfig struct {
	config.Configuration
	webhook config.WebhookConfig
}

func (f *fakeConfig) GetWebhook() config.WebhookConfig { return f.webhook }

func newTestController(webhookCfg config.WebhookConfig) *controller {
	return &controller{
		config:   &fakeConfig{webhook: webhookCfg},
		nsLister: &fakeNamespaceLister{namespaces: map[string]*corev1.Namespace{}},
	}
}

func nsObj(name string, lbls map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: lbls},
	}
}

// makeValidatingPolicy wraps a ValidatingPolicy into a GenericPolicy.
func makeValidatingPolicy(name string, nsSel, objSel *metav1.LabelSelector) engineapi.GenericPolicy {
	vpol := &policiesv1beta1api.ValidatingPolicy{}
	vpol.Name = name
	vpol.Spec.MatchConstraints = &admissionregistrationv1.MatchResources{
		NamespaceSelector: nsSel,
		ObjectSelector:    objSel,
	}
	return engineapi.NewValidatingPolicy(vpol)
}

// ---------------------------------------------------------------------------
// mergeLabelSelectors
// ---------------------------------------------------------------------------

func TestMergeLabelSelectors_BothNil(t *testing.T) {
	assert.Nil(t, mergeLabelSelectors(nil, nil))
}

func TestMergeLabelSelectors_ANil(t *testing.T) {
	b := &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}}
	assert.Equal(t, b, mergeLabelSelectors(nil, b))
}

func TestMergeLabelSelectors_BNil(t *testing.T) {
	a := &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}}
	assert.Equal(t, a, mergeLabelSelectors(a, nil))
}

func TestMergeLabelSelectors_BothSet(t *testing.T) {
	a := &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}}
	b := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}}
	merged := mergeLabelSelectors(a, b)
	assert.Equal(t, "prod", merged.MatchLabels["env"])
	assert.Equal(t, "foo", merged.MatchLabels["app"])
}

func TestMergeLabelSelectors_ExpressionsAppended(t *testing.T) {
	req1 := metav1.LabelSelectorRequirement{
		Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{"prod"},
	}
	req2 := metav1.LabelSelectorRequirement{
		Key: "app", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"test"},
	}
	a := &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{req1}}
	b := &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{req2}}
	merged := mergeLabelSelectors(a, b)
	assert.Len(t, merged.MatchExpressions, 2)
}

// ---------------------------------------------------------------------------
// policyWebhookSelectors
// ---------------------------------------------------------------------------

func TestPolicyWebhookSelectors_ValidatingPolicy(t *testing.T) {
	nsSel := &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}}
	objSel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}}
	policy := makeValidatingPolicy("test", nsSel, objSel)

	gotNs, gotObj := policyWebhookSelectors(policy)
	assert.Equal(t, nsSel, gotNs)
	assert.Equal(t, objSel, gotObj)
}

func TestPolicyWebhookSelectors_ValidatingPolicyNilSelectors(t *testing.T) {
	policy := makeValidatingPolicy("test", nil, nil)
	gotNs, gotObj := policyWebhookSelectors(policy)
	assert.Nil(t, gotNs)
	assert.Nil(t, gotObj)
}

// ---------------------------------------------------------------------------
// filterPoliciesByWebhookSelectors — global selector only (v1 policy returns nil)
// ---------------------------------------------------------------------------

func TestFilterPolicies_NoPolicies(t *testing.T) {
	c := newTestController(config.WebhookConfig{})
	result, err := c.filterPoliciesByWebhookSelectors(
		map[string]string{"env": "prod"},
		map[string]string{"app": "foo"},
		nil,
	)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestFilterPolicies_GlobalNsSelectorMatches(t *testing.T) {
	// Global namespace selector: env=prod; namespace has env=prod → included
	c := newTestController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "prod"},
		},
	})
	policy := makeValidatingPolicy("p1", nil, nil)
	result, err := c.filterPoliciesByWebhookSelectors(
		map[string]string{"env": "prod"},
		map[string]string{},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestFilterPolicies_GlobalNsSelectorExcludes(t *testing.T) {
	// Global namespace selector: env=prod; namespace has env=dev → excluded
	c := newTestController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "prod"},
		},
	})
	policy := makeValidatingPolicy("p1", nil, nil)
	result, err := c.filterPoliciesByWebhookSelectors(
		map[string]string{"env": "dev"},
		map[string]string{},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestFilterPolicies_GlobalObjSelectorMatches(t *testing.T) {
	c := newTestController(config.WebhookConfig{
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "foo"},
		},
	})
	policy := makeValidatingPolicy("p1", nil, nil)
	result, err := c.filterPoliciesByWebhookSelectors(
		nil,
		map[string]string{"app": "foo"},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestFilterPolicies_GlobalObjSelectorExcludes(t *testing.T) {
	c := newTestController(config.WebhookConfig{
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "foo"},
		},
	})
	policy := makeValidatingPolicy("p1", nil, nil)
	result, err := c.filterPoliciesByWebhookSelectors(
		nil,
		map[string]string{"app": "bar"},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// filterPoliciesByWebhookSelectors — per-policy selectors
// ---------------------------------------------------------------------------

func TestFilterPolicies_PerPolicyNsSelectorExcludes(t *testing.T) {
	// Policy has its own namespace selector: env=prod; namespace has env=dev → excluded
	// Global config has no selector, so effective = policy selector only.
	c := newTestController(config.WebhookConfig{})
	nsSel := &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}}
	policy := makeValidatingPolicy("p1", nsSel, nil)

	result, err := c.filterPoliciesByWebhookSelectors(
		map[string]string{"env": "dev"},
		map[string]string{},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestFilterPolicies_PerPolicyNsSelectorMatches(t *testing.T) {
	c := newTestController(config.WebhookConfig{})
	nsSel := &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}}
	policy := makeValidatingPolicy("p1", nsSel, nil)

	result, err := c.filterPoliciesByWebhookSelectors(
		map[string]string{"env": "prod"},
		map[string]string{},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestFilterPolicies_MergedSelectorsPartialMatch(t *testing.T) {
	// Policy p1: ns selector env=prod → excluded for dev namespace
	// Policy p2: no ns selector, no global → always included
	// → only p2 survives
	c := newTestController(config.WebhookConfig{})
	p1 := makeValidatingPolicy("p1", &metav1.LabelSelector{
		MatchLabels: map[string]string{"env": "prod"},
	}, nil)
	p2 := makeValidatingPolicy("p2", nil, nil)

	result, err := c.filterPoliciesByWebhookSelectors(
		map[string]string{"env": "dev"},
		map[string]string{},
		[]engineapi.GenericPolicy{p1, p2},
	)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "p2", result[0].GetName())
}

func TestFilterPolicies_GlobalAndPerPolicyBothMustMatch(t *testing.T) {
	// Global ns selector: tier=backend
	// Policy ns selector: env=prod
	// Effective = env=prod AND tier=backend
	// Namespace has env=prod but NOT tier=backend → excluded
	c := newTestController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"tier": "backend"},
		},
	})
	policy := makeValidatingPolicy("p1", &metav1.LabelSelector{
		MatchLabels: map[string]string{"env": "prod"},
	}, nil)

	result, err := c.filterPoliciesByWebhookSelectors(
		map[string]string{"env": "prod"},
		map[string]string{},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestFilterPolicies_NilNsLabelsSkipsNsCheck(t *testing.T) {
	// nsLabels=nil means cluster-scoped resource → namespace selector is skipped
	c := newTestController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "prod"},
		},
	})
	policy := makeValidatingPolicy("p1", nil, nil)

	result, err := c.filterPoliciesByWebhookSelectors(
		nil, // cluster-scoped: no namespace labels
		map[string]string{},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestFilterPolicies_NotInExpression(t *testing.T) {
	// Common pattern: exclude kube-system via NotIn expression
	c := newTestController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "kubernetes.io/metadata.name",
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"kube-system", "kube-public"},
				},
			},
		},
	})
	policy := makeValidatingPolicy("p1", nil, nil)

	// kube-system → excluded
	result, err := c.filterPoliciesByWebhookSelectors(
		map[string]string{"kubernetes.io/metadata.name": "kube-system"},
		map[string]string{},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Empty(t, result)

	// default → included
	result, err = c.filterPoliciesByWebhookSelectors(
		map[string]string{"kubernetes.io/metadata.name": "default"},
		map[string]string{},
		[]engineapi.GenericPolicy{policy},
	)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}
