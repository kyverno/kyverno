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
	"k8s.io/utils/ptr"
)

func TestExcludeBootstrapResourcesFromValidatingWebhooks(t *testing.T) {
	userCondition := admissionregistrationv1.MatchCondition{Name: "user", Expression: "true"}
	build := func() []admissionregistrationv1.ValidatingWebhook {
		return []admissionregistrationv1.ValidatingWebhook{
			{Name: "resource-fail", FailurePolicy: ptr.To(admissionregistrationv1.Fail)},
			{Name: "resource-ignore", FailurePolicy: ptr.To(admissionregistrationv1.Ignore)},
			{
				Name:            "resource-fail-with-existing",
				FailurePolicy:   ptr.To(admissionregistrationv1.Fail),
				MatchConditions: []admissionregistrationv1.MatchCondition{userCondition},
			},
		}
	}

	t.Run("disabled is a no-op", func(t *testing.T) {
		webhooks := build()
		excludeBootstrapResourcesFromValidatingWebhooks(webhooks, false)
		assert.Empty(t, webhooks[0].MatchConditions)
		assert.Empty(t, webhooks[1].MatchConditions)
		assert.Len(t, webhooks[2].MatchConditions, 1)
	})

	t.Run("enabled appends the exclusion to Fail webhooks only", func(t *testing.T) {
		webhooks := build()
		excludeBootstrapResourcesFromValidatingWebhooks(webhooks, true)

		// Fail webhook gets the exclusion appended.
		assert.Len(t, webhooks[0].MatchConditions, 1)
		assert.Equal(t, bootstrapExclusionMatchConditionName, webhooks[0].MatchConditions[0].Name)
		assert.Equal(t, bootstrapExclusionExpression, webhooks[0].MatchConditions[0].Expression)

		// Ignore webhook is untouched (it already fails open, so it cannot deadlock).
		assert.Empty(t, webhooks[1].MatchConditions)

		// Existing user match conditions are preserved, with the exclusion appended after.
		assert.Len(t, webhooks[2].MatchConditions, 2)
		assert.Equal(t, userCondition, webhooks[2].MatchConditions[0])
		assert.Equal(t, bootstrapExclusionMatchConditionName, webhooks[2].MatchConditions[1].Name)
	})
}

func TestExcludeBootstrapResourcesFromMutatingWebhooks(t *testing.T) {
	build := func() []admissionregistrationv1.MutatingWebhook {
		return []admissionregistrationv1.MutatingWebhook{
			{Name: "resource-fail", FailurePolicy: ptr.To(admissionregistrationv1.Fail)},
			{Name: "resource-ignore", FailurePolicy: ptr.To(admissionregistrationv1.Ignore)},
		}
	}

	t.Run("disabled is a no-op", func(t *testing.T) {
		webhooks := build()
		excludeBootstrapResourcesFromMutatingWebhooks(webhooks, false)
		assert.Empty(t, webhooks[0].MatchConditions)
		assert.Empty(t, webhooks[1].MatchConditions)
	})

	t.Run("enabled appends the exclusion to Fail webhooks only", func(t *testing.T) {
		webhooks := build()
		excludeBootstrapResourcesFromMutatingWebhooks(webhooks, true)
		assert.Len(t, webhooks[0].MatchConditions, 1)
		assert.Equal(t, bootstrapExclusionMatchConditionName, webhooks[0].MatchConditions[0].Name)
		assert.Empty(t, webhooks[1].MatchConditions)
	})
}

// TestBootstrapExclusionExpression pins the CEL expression to the form the
// Kubernetes docs document for skipping cluster-scoped resources, so an
// accidental edit that would silently stop matching Node/CSR fails the build.
func TestBootstrapExclusionExpression(t *testing.T) {
	assert.Equal(t,
		`!(request.resource.group == "" && request.resource.resource == "nodes") && !(request.resource.group == "certificates.k8s.io" && request.resource.resource == "certificatesigningrequests")`,
		bootstrapExclusionExpression,
	)
}

func TestBootstrapExclusionMatchConditions(t *testing.T) {
	assert.Nil(t, bootstrapExclusionMatchConditions(false))

	conditions := bootstrapExclusionMatchConditions(true)
	assert.Len(t, conditions, 1)
	assert.Equal(t, bootstrapExclusionMatchConditionName, conditions[0].Name)
	assert.Equal(t, bootstrapExclusionExpression, conditions[0].Expression)
}

// TestBootstrapExclusionExpressionCompiles verifies the expression is valid CEL
// under the Kubernetes match-condition environment, i.e. the API server would
// accept it. An invalid expression would be rejected at webhook registration.
func TestBootstrapExclusionExpressionCompiles(t *testing.T) {
	cache := NewExpressionCache()
	for _, mc := range bootstrapExclusionMatchConditions(true) {
		compiled := cache.GetOrCompile(mc)
		assert.True(t, compiled.isValid, "expression must compile: %v", compiled.errors)
	}
}

// TestBuildDefaultResourceValidatingWebhookConfiguration_ExcludesBootstrapResources
// confirms the wiring end to end: with the flag on, the static wildcard webhook
// config (autoUpdateWebhooks=false) emits the exclusion on its Fail webhook and
// leaves the Ignore webhook untouched.
func TestBuildDefaultResourceValidatingWebhookConfiguration_ExcludesBootstrapResources(t *testing.T) {
	c := &controller{
		defaultTimeout:            10,
		servicePort:               443,
		excludeBootstrapResources: true,
		clusterroleLister:         rbacv1listers.NewClusterRoleLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})),
	}

	vwc, err := c.buildDefaultResourceValidatingWebhookConfiguration(context.TODO(), config.NewDefaultConfiguration(false), nil)
	require.NoError(t, err)

	byName := make(map[string]admissionregistrationv1.ValidatingWebhook, len(vwc.Webhooks))
	for _, w := range vwc.Webhooks {
		byName[w.Name] = w
	}

	failWebhook, ok := byName[config.ValidatingWebhookName+"-fail"]
	require.True(t, ok, "fail webhook must exist")
	require.Len(t, failWebhook.MatchConditions, 1)
	assert.Equal(t, bootstrapExclusionMatchConditionName, failWebhook.MatchConditions[0].Name)

	ignoreWebhook, ok := byName[config.ValidatingWebhookName+"-ignore"]
	require.True(t, ok, "ignore webhook must exist")
	assert.Empty(t, ignoreWebhook.MatchConditions)
}
