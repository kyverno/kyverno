package background

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
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

func newController(webhookCfg config.WebhookConfig, namespaces map[string]*corev1.Namespace) *controller {
	return &controller{
		config:   &fakeConfig{webhook: webhookCfg},
		nsLister: &fakeNamespaceLister{namespaces: namespaces},
	}
}

func ns(name string, lbls map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: lbls,
		},
	}
}

// ---------------------------------------------------------------------------
// isNamespaceExcludedByWebhookSelector
// ---------------------------------------------------------------------------

func TestIsNamespaceExcludedByWebhookSelector_NilSelector(t *testing.T) {
	c := newController(config.WebhookConfig{NamespaceSelector: nil}, map[string]*corev1.Namespace{
		"default": ns("default", map[string]string{"env": "prod"}),
	})
	excluded, err := c.isNamespaceExcludedByWebhookSelector("default")
	assert.NoError(t, err)
	assert.False(t, excluded)
}

func TestIsNamespaceExcludedByWebhookSelector_Matches(t *testing.T) {
	// Selector: env=prod → webhook applies → NOT excluded
	c := newController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "prod"},
		},
	}, map[string]*corev1.Namespace{
		"default": ns("default", map[string]string{"env": "prod"}),
	})
	excluded, err := c.isNamespaceExcludedByWebhookSelector("default")
	assert.NoError(t, err)
	assert.False(t, excluded)
}

func TestIsNamespaceExcludedByWebhookSelector_NoMatch(t *testing.T) {
	// Selector: env=prod but namespace has env=dev → webhook does NOT apply → excluded
	c := newController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "prod"},
		},
	}, map[string]*corev1.Namespace{
		"staging": ns("staging", map[string]string{"env": "dev"}),
	})
	excluded, err := c.isNamespaceExcludedByWebhookSelector("staging")
	assert.NoError(t, err)
	assert.True(t, excluded)
}

func TestIsNamespaceExcludedByWebhookSelector_MatchExpressionNotIn(t *testing.T) {
	// Common pattern: exclude kube-system via NotIn
	c := newController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "kubernetes.io/metadata.name",
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"kube-system", "kube-public"},
				},
			},
		},
	}, map[string]*corev1.Namespace{
		"kube-system": ns("kube-system", map[string]string{"kubernetes.io/metadata.name": "kube-system"}),
		"default":     ns("default", map[string]string{"kubernetes.io/metadata.name": "default"}),
	})

	excluded, err := c.isNamespaceExcludedByWebhookSelector("kube-system")
	assert.NoError(t, err)
	assert.True(t, excluded, "kube-system should be excluded")

	excluded, err = c.isNamespaceExcludedByWebhookSelector("default")
	assert.NoError(t, err)
	assert.False(t, excluded, "default should not be excluded")
}

func TestIsNamespaceExcludedByWebhookSelector_NamespaceNotFound(t *testing.T) {
	c := newController(config.WebhookConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"env": "prod"},
		},
	}, map[string]*corev1.Namespace{})
	_, err := c.isNamespaceExcludedByWebhookSelector("missing")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// isObjectExcludedByWebhookSelector
// ---------------------------------------------------------------------------

func TestIsObjectExcludedByWebhookSelector_NilSelector(t *testing.T) {
	c := newController(config.WebhookConfig{ObjectSelector: nil}, nil)
	excluded, err := c.isObjectExcludedByWebhookSelector(map[string]string{"app": "foo"})
	assert.NoError(t, err)
	assert.False(t, excluded)
}

func TestIsObjectExcludedByWebhookSelector_Matches(t *testing.T) {
	// Selector: app=foo → webhook applies → NOT excluded
	c := newController(config.WebhookConfig{
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "foo"},
		},
	}, nil)
	excluded, err := c.isObjectExcludedByWebhookSelector(map[string]string{"app": "foo"})
	assert.NoError(t, err)
	assert.False(t, excluded)
}

func TestIsObjectExcludedByWebhookSelector_NoMatch(t *testing.T) {
	// Selector: app=foo but resource has app=bar → excluded
	c := newController(config.WebhookConfig{
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "foo"},
		},
	}, nil)
	excluded, err := c.isObjectExcludedByWebhookSelector(map[string]string{"app": "bar"})
	assert.NoError(t, err)
	assert.True(t, excluded)
}

func TestIsObjectExcludedByWebhookSelector_EmptyResourceLabels(t *testing.T) {
	// Selector requires app=foo; resource has no labels → excluded
	c := newController(config.WebhookConfig{
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "foo"},
		},
	}, nil)
	excluded, err := c.isObjectExcludedByWebhookSelector(map[string]string{})
	assert.NoError(t, err)
	assert.True(t, excluded)
}

func TestIsObjectExcludedByWebhookSelector_EmptySelectorMatchesAll(t *testing.T) {
	// Empty selector matches everything → nothing excluded
	c := newController(config.WebhookConfig{
		ObjectSelector: &metav1.LabelSelector{},
	}, nil)
	excluded, err := c.isObjectExcludedByWebhookSelector(map[string]string{})
	assert.NoError(t, err)
	assert.False(t, excluded)
}
