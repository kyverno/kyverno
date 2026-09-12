package webhook

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

// findPolicyRule locates the RuleWithOperations whose Rule targets the legacy
// kyverno.io ClusterPolicy/Policy kinds among the given rules.
func findPolicyRule(t *testing.T, rules []admissionregistrationv1.RuleWithOperations) admissionregistrationv1.Rule {
	t.Helper()
	for _, r := range rules {
		if len(r.Rule.APIGroups) == 1 && r.Rule.APIGroups[0] == "kyverno.io" &&
			len(r.Rule.Resources) == 2 && r.Rule.Resources[0] == "clusterpolicies" && r.Rule.Resources[1] == "policies" {
			return r.Rule
		}
	}
	t.Fatalf("no rule found for kyverno.io clusterpolicies/policies")
	return admissionregistrationv1.Rule{}
}

// TestPolicyValidatingWebhookPreservesLegacyAPIVersions is a 1.20 regression
// guard for #17491: legacy kyverno.io create/update requests must keep
// reaching the engine so the intended hard error (see
// pkg/deprecations.BuildKindError) is still raised. This test fails if
// controller.go's policyRule.APIVersions is changed to drop "v1" or
// "v2beta1" before the 1.21 removal.
func TestPolicyValidatingWebhookPreservesLegacyAPIVersions(t *testing.T) {
	c := &controller{
		defaultTimeout:    10,
		servicePort:       443,
		clusterroleLister: rbacv1listers.NewClusterRoleLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
	}

	vwc, err := c.buildPolicyValidatingWebhookConfiguration(context.TODO(), config.NewDefaultConfiguration(false), nil)
	require.NoError(t, err)
	require.Len(t, vwc.Webhooks, 1)

	rule := findPolicyRule(t, vwc.Webhooks[0].Rules)
	assert.Equal(t, []string{"v1", "v2beta1"}, rule.APIVersions)
}

// TestPolicyMutatingWebhookPreservesLegacyAPIVersions mirrors
// TestPolicyValidatingWebhookPreservesLegacyAPIVersions for the mutating
// policy webhook configuration.
func TestPolicyMutatingWebhookPreservesLegacyAPIVersions(t *testing.T) {
	c := &controller{
		defaultTimeout:    10,
		servicePort:       443,
		clusterroleLister: rbacv1listers.NewClusterRoleLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
	}

	mwc, err := c.buildPolicyMutatingWebhookConfiguration(context.TODO(), config.NewDefaultConfiguration(false), nil)
	require.NoError(t, err)
	require.Len(t, mwc.Webhooks, 1)

	rule := findPolicyRule(t, mwc.Webhooks[0].Rules)
	assert.Equal(t, []string{"v1", "v2beta1"}, rule.APIVersions)
}

// TestPolicyRuleAPIVersions is a cheap secondary guard directly on the
// package-level policyRule variable.
func TestPolicyRuleAPIVersions(t *testing.T) {
	assert.Equal(t, []string{"v1", "v2beta1"}, policyRule.APIVersions)
}
