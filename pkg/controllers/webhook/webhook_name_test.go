package webhook

import (
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// fineGrainedSpec has a match condition, so the policy gets its own fine-grained webhook.
func fineGrainedSpec() policiesv1beta1.ValidatingPolicySpec {
	return policiesv1beta1.ValidatingPolicySpec{
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
					},
				},
			}},
		},
		MatchConditions: []admissionregistrationv1.MatchCondition{{
			Name:       "only-labeled",
			Expression: "has(object.metadata.labels)",
		}},
	}
}

func fineGrainedWebhookNames(t *testing.T, webhookName, queryPath string, policies ...engineapi.GenericPolicy) []string {
	t.Helper()
	expressionCache := NewExpressionCache()
	for _, p := range policies {
		expressionCache.AddPolicyExpressions(extractGenericPolicy(p).GetMatchConditions())
	}
	webhooks := buildWebhookRules(config.NewDefaultConfiguration(false), "", webhookName, queryPath, 0, nil, policies, expressionCache)
	names := make([]string, 0, len(webhooks))
	for _, w := range webhooks {
		names = append(names, w.Name)
	}
	return names
}

// TestBuildWebhookRules_FineGrainedWebhookNameLength: a fine-grained webhook is named after its
// policy (and namespace). The API server rejects a webhook configuration with any webhook name
// over 253 characters, and every policy kind's webhooks share one configuration, so one long
// policy name would stop every other policy's webhook from being registered.
func TestBuildWebhookRules_FineGrainedWebhookNameLength(t *testing.T) {
	// valid object names: a namespace is at most 63 characters, a policy name at most 253
	longNamespace := "payments-ledger-reconciliation-svc-production-eu-central-1"
	longName := "payments-ledger-reconciliation-svc-require-approved-labels-on-configmaps-managed-by-argocd-applicationset-payments-ledger-reconciliation-svc-production-eu-central-1-tenant-a"
	require.Empty(t, validation.IsDNS1123Label(longNamespace))
	require.Empty(t, validation.IsDNS1123Subdomain(longName))

	nvpol := func(namespace, name string) engineapi.GenericPolicy {
		return engineapi.NewNamespacedValidatingPolicy(&policiesv1beta1.NamespacedValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       fineGrainedSpec(),
		})
	}
	vpol := func(name string) engineapi.GenericPolicy {
		return engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       fineGrainedSpec(),
		})
	}
	assertValid := func(t *testing.T, names []string) {
		t.Helper()
		for i, name := range names {
			errs := validation.IsFullyQualifiedName(field.NewPath("webhooks").Index(i).Child("name"), name)
			assert.Empty(t, errs, "the API server would reject this webhook name: %v", errs.ToAggregate())
		}
	}

	t.Run("short names are unchanged", func(t *testing.T) {
		names := fineGrainedWebhookNames(t, config.NamespacedValidatingPolicyWebhookName, "/nvpol", nvpol("team-a", "require-labels"))
		assert.Equal(t, []string{config.NamespacedValidatingPolicyWebhookName + "-fail-finegrained-team-a-require-labels"}, names)
	})

	t.Run("long namespaced policy name stays within the limit", func(t *testing.T) {
		names := fineGrainedWebhookNames(t, config.NamespacedValidatingPolicyWebhookName, "/nvpol", nvpol(longNamespace, longName))
		require.Len(t, names, 1)
		assert.Len(t, names[0], 253)
		assert.True(t, strings.HasPrefix(names[0], config.NamespacedValidatingPolicyWebhookName+"-fail-finegrained-"+longNamespace+"-"))
		assertValid(t, names)
	})

	t.Run("long cluster policy name stays within the limit", func(t *testing.T) {
		names := fineGrainedWebhookNames(t, config.ValidatingPolicyWebhookName, "/vpol", vpol(strings.Repeat("a", 253)))
		require.Len(t, names, 1)
		assertValid(t, names)
	})

	t.Run("long names that share a prefix stay unique and deterministic", func(t *testing.T) {
		first := nvpol(longNamespace, longName+"-one")
		second := nvpol(longNamespace, longName+"-two")
		names := fineGrainedWebhookNames(t, config.NamespacedValidatingPolicyWebhookName, "/nvpol", first, second)
		require.Len(t, names, 2)
		assert.NotEqual(t, names[0], names[1])
		assertValid(t, names)
		assert.Equal(t, names, fineGrainedWebhookNames(t, config.NamespacedValidatingPolicyWebhookName, "/nvpol", first, second))
	})
}

// TestGenerateName_TruncationBoundary pins down the exact limit and the trimming at the cut.
// The hash suffix is 16 characters plus a separating dash, so a truncated name keeps the first
// 253-17 = 236 characters of the full name, minus any trailing '-' or '.'.
func TestGenerateName_TruncationBoundary(t *testing.T) {
	const prefix = "w" // full name = "w-" + policy name
	const keep = maxWebhookNameLength - 17
	// policyName returns a valid policy name whose full webhook name is total characters long,
	// with the characters at the given full-name positions overridden.
	policyName := func(total int, at map[int]byte) string {
		b := []byte(strings.Repeat("a", total-len(prefix)-1))
		for i, c := range at {
			b[i-len(prefix)-1] = c
		}
		return string(b)
	}
	tests := []struct {
		name string
		// policy name, built so that the full name has the length the test case needs
		policy string
		// whether the name must come back unchanged
		unchanged bool
		// length of the kept prefix before "-<hash>"
		wantKept int
	}{{
		name:      "exactly 253 characters is unchanged",
		policy:    policyName(253, nil),
		unchanged: true,
	}, {
		name:     "254 characters is truncated and hashed",
		policy:   policyName(254, nil),
		wantKept: keep,
	}, {
		name:     "a dot at the cut is trimmed",
		policy:   policyName(255, map[int]byte{keep - 1: '.'}),
		wantKept: keep - 1,
	}, {
		name:     "dashes at the cut are trimmed",
		policy:   policyName(255, map[int]byte{keep - 2: '-', keep - 1: '-'}),
		wantKept: keep - 2,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Empty(t, validation.IsDNS1123Subdomain(tt.policy), "test input must be a valid policy name")
			full := prefix + "-" + tt.policy
			got := generateName(prefix, &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: tt.policy}})

			assert.Empty(t, validation.IsDNS1123Subdomain(got), "must be a valid webhook name (length and characters)")
			if tt.unchanged {
				assert.Equal(t, full, got)
				return
			}
			assert.Len(t, got, tt.wantKept+17)
			assert.Equal(t, full[:tt.wantKept], got[:tt.wantKept], "keeps the start of the full name")
			assert.Equal(t, byte('-'), got[tt.wantKept], "separates the hash with a single dash")
			assert.Regexp(t, `^[0-9a-f]{16}$`, got[tt.wantKept+1:], "ends with the hash")
			assert.Equal(t, got, generateName(prefix, &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: tt.policy}}), "deterministic")
		})
	}
}
