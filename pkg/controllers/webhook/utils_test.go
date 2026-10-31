package webhook

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

func Test_webhook_isEmpty(t *testing.T) {
	empty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore, []admissionregistrationv1.MatchCondition{})
	assert.Equal(t, empty.isEmpty(), true)
	notEmpty := newWebhook(DefaultWebhookTimeout, admissionregistrationv1.Ignore, []admissionregistrationv1.MatchCondition{})
	notEmpty.set("", "v1", "pods", "", admissionregistrationv1.NamespacedScope, admissionv1.Create)
	assert.Equal(t, notEmpty.isEmpty(), false)
}

func TestBuildRulesWithOperations(t *testing.T) {
	testCases := []struct {
		name           string
		rules          sets.Set[ruleEntry]
		expectedResult []admissionregistrationv1.RuleWithOperations
	}{{
		rules: sets.New[ruleEntry](
			ruleEntry{"", "v1", "configmaps", "", admissionregistrationv1.NamespacedScope, admissionv1.Create},
			ruleEntry{"", "v1", "pods", "", admissionregistrationv1.NamespacedScope, admissionv1.Create},
		),
		expectedResult: []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"configmaps", "pods", "pods/ephemeralcontainers"},
				Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
			},
		}},
	}, {
		rules: sets.New[ruleEntry](
			ruleEntry{"", "v1", "configmaps", "", admissionregistrationv1.NamespacedScope, admissionv1.Create},
			ruleEntry{"", "v1", "pods", "", admissionregistrationv1.NamespacedScope, admissionv1.Create},
			ruleEntry{"", "v1", "pods", "", admissionregistrationv1.NamespacedScope, admissionv1.Update},
		),
		expectedResult: []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"configmaps"},
				Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
			},
		}, {
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"pods", "pods/ephemeralcontainers"},
				Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
			},
		}},
	}}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			wh := &webhook{
				rules: testCase.rules,
			}
			result := wh.buildRulesWithOperations()
			assert.Equal(t, testCase.expectedResult, result)
		})
	}
}

func Test_less(t *testing.T) {
	tests := []struct {
		name string
		do   func() int
		want int
	}{{
		do: func() int {
			return less([]int{0}, []int{0, 0})
		},
		want: -1,
	}, {
		do: func() int {
			return less([]int{0, 0}, []int{0})
		},
		want: 1,
	}, {
		do: func() int {
			return less([]int{0}, []int{1})
		},
		want: -1,
	}, {
		do: func() int {
			return less([]int{1}, []int{0})
		},
		want: 1,
	}, {
		do: func() int {
			return less([]int{0, 0}, []int{0, 0})
		},
		want: 0,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.do()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDeduplicateRules(t *testing.T) {
	rule1 := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps", ""},
			APIVersions: []string{"v1"},
			Resources:   []string{"pods", "deployments"},
			Scope:       ptr.To(admissionregistrationv1.AllScopes),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
	}
	rule2_ExactDuplicate := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps", ""},
			APIVersions: []string{"v1"},
			Resources:   []string{"pods", "deployments"},
			Scope:       ptr.To(admissionregistrationv1.AllScopes),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
	}
	rule3_LogicalDuplicate := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"", "apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"deployments", "pods"},
			Scope:       ptr.To(admissionregistrationv1.AllScopes),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Update,
			admissionregistrationv1.Create,
		},
	}
	rule4_Unique := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"statefulsets"},
			Scope:       ptr.To(admissionregistrationv1.AllScopes),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
	}
	rule5_AnotherUnique := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"batch"},
			APIVersions: []string{"v1"},
			Resources:   []string{"jobs", "cronjobs"},
			Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Delete,
		},
	}
	rule6_LogicalDuplicateOfRule5 := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"batch"},
			APIVersions: []string{"v1"},
			Resources:   []string{"cronjobs", "jobs"},
			Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Delete,
			admissionregistrationv1.Create,
		},
	}
	rule7_ExactDuplicateOfRule4 := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"statefulsets"},
			Scope:       ptr.To(admissionregistrationv1.AllScopes),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
	}
	rule8_NilScope := *rule1.DeepCopy()
	rule8_NilScope.Scope = nil

	testCases := []struct {
		name          string
		input         []admissionregistrationv1.RuleWithOperations
		expectedCount int
		expectedRules []admissionregistrationv1.RuleWithOperations
	}{
		{
			name:          "Nil and explicit AllScopes are duplicates",
			input:         []admissionregistrationv1.RuleWithOperations{rule8_NilScope, rule1},
			expectedCount: 1,
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule8_NilScope},
		},
		{
			name:          "Explicit AllScopes and nil are duplicates",
			input:         []admissionregistrationv1.RuleWithOperations{rule1, rule8_NilScope},
			expectedCount: 1,
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule1},
		},
		{
			name:          "No duplicates",
			input:         []admissionregistrationv1.RuleWithOperations{rule1, rule4_Unique},
			expectedCount: 2,
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule1, rule4_Unique},
		},
		{
			name:          "Simple exact duplicate",
			input:         []admissionregistrationv1.RuleWithOperations{rule1, rule2_ExactDuplicate},
			expectedCount: 1,
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule1},
		},
		{
			name:          "Logical duplicate (different slice order)",
			input:         []admissionregistrationv1.RuleWithOperations{rule1, rule3_LogicalDuplicate},
			expectedCount: 1,
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule1},
		},
		{
			name:          "All types of duplicates and unique rules",
			input:         []admissionregistrationv1.RuleWithOperations{rule1, rule2_ExactDuplicate, rule3_LogicalDuplicate, rule4_Unique},
			expectedCount: 2,
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule1, rule4_Unique},
		},
		{
			name:          "Multiple unique rules with mixed duplicates",
			input:         []admissionregistrationv1.RuleWithOperations{rule1, rule4_Unique, rule5_AnotherUnique, rule6_LogicalDuplicateOfRule5, rule7_ExactDuplicateOfRule4},
			expectedCount: 3,
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule1, rule4_Unique, rule5_AnotherUnique},
		},
		{
			name:          "Empty list",
			input:         []admissionregistrationv1.RuleWithOperations{},
			expectedCount: 0,
			expectedRules: []admissionregistrationv1.RuleWithOperations{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := deDuplicatedRules(tc.input)
			assert.Equal(t, tc.expectedCount, len(result))
			assert.Equal(t, tc.expectedRules, result)
		})
	}
}

func TestSortedRules(t *testing.T) {
	rule_apps_pods := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"pods"},
			Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
	}
	rule_apps_pods_cluster := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"pods"},
			Scope:       ptr.To(admissionregistrationv1.ClusterScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
	}
	rule_apps_pods_all := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"pods"},
			Scope:       ptr.To(admissionregistrationv1.AllScopes),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.OperationAll,
		},
	}
	rule_batch_jobs := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"batch"},
			APIVersions: []string{"v1"},
			Resources:   []string{"jobs"},
			Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Delete,
		},
	}
	rule_batch_cronjobs := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"batch"},
			APIVersions: []string{"v1"},
			Resources:   []string{"cronjobs"},
			Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Delete,
		},
	}
	rule_v1beta1_apps := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1beta1"},
			Resources:   []string{"deployments", "pods"},
			Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
			admissionregistrationv1.Update,
		},
	}

	testCases := []struct {
		name          string
		input         []admissionregistrationv1.RuleWithOperations
		expectedCount int
		expectedRules []admissionregistrationv1.RuleWithOperations
	}{
		{
			name:          "Sorted by API group",
			input:         []admissionregistrationv1.RuleWithOperations{rule_batch_jobs, rule_apps_pods},
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule_apps_pods, rule_batch_jobs},
		},
		{
			name:          "Sorted by API version",
			input:         []admissionregistrationv1.RuleWithOperations{rule_v1beta1_apps, rule_apps_pods},
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule_apps_pods, rule_v1beta1_apps},
		},
		{
			name:          "Sorted by resource",
			input:         []admissionregistrationv1.RuleWithOperations{rule_batch_jobs, rule_batch_cronjobs},
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule_batch_cronjobs, rule_batch_jobs},
		},
		{
			name:          "Sorted by operation",
			input:         []admissionregistrationv1.RuleWithOperations{rule_apps_pods, rule_apps_pods_all},
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule_apps_pods_all, rule_apps_pods},
		},
		{
			name:          "Sorted by scope",
			input:         []admissionregistrationv1.RuleWithOperations{rule_apps_pods, rule_apps_pods_cluster},
			expectedRules: []admissionregistrationv1.RuleWithOperations{rule_apps_pods_cluster, rule_apps_pods},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sortedRules(tc.input)
			assert.Equal(t, tc.expectedRules, result)
		})
	}
}

func TestSortedRules_NilScope(t *testing.T) {
	ruleNilScope := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"deployments"},
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
		},
	}
	ruleNamespacedScope := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"deployments"},
			Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
		},
	}
	ruleClusterScope := admissionregistrationv1.RuleWithOperations{
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"deployments"},
			Scope:       ptr.To(admissionregistrationv1.ClusterScope),
		},
		Operations: []admissionregistrationv1.OperationType{
			admissionregistrationv1.Create,
		},
	}

	t.Run("nil scope does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			sortedRules([]admissionregistrationv1.RuleWithOperations{ruleNilScope, ruleNamespacedScope})
		})
	})

	t.Run("both nil scopes do not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			sortedRules([]admissionregistrationv1.RuleWithOperations{ruleNilScope, ruleNilScope})
		})
	})

	t.Run("nil scope treated as AllScopes sorts before Cluster and Namespaced", func(t *testing.T) {
		result := sortedRules([]admissionregistrationv1.RuleWithOperations{
			ruleNamespacedScope, ruleNilScope, ruleClusterScope,
		})
		assert.Equal(t, string(admissionregistrationv1.AllScopes), scopeString(result[0].Scope))
		assert.Equal(t, string(admissionregistrationv1.ClusterScope), scopeString(result[1].Scope))
		assert.Equal(t, string(admissionregistrationv1.NamespacedScope), scopeString(result[2].Scope))
	})
}

func TestScopeString(t *testing.T) {
	tests := []struct {
		name     string
		input    *admissionregistrationv1.ScopeType
		expected string
	}{
		{"nil returns AllScopes", nil, string(admissionregistrationv1.AllScopes)},
		{"Namespaced", ptr.To(admissionregistrationv1.NamespacedScope), string(admissionregistrationv1.NamespacedScope)},
		{"Cluster", ptr.To(admissionregistrationv1.ClusterScope), string(admissionregistrationv1.ClusterScope)},
		{"AllScopes", ptr.To(admissionregistrationv1.AllScopes), string(admissionregistrationv1.AllScopes)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, scopeString(tt.input))
		})
	}
}

func TestIvpolsNeedingMutation(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name          string
		ivpols        []engineapi.GenericPolicy
		expectedCount int
	}{
		{
			name:          "empty input",
			ivpols:        []engineapi.GenericPolicy{},
			expectedCount: 0,
		},
		{
			name: "nil MutateDigest and nil VerifyDigest default to true -- needs mutation",
			ivpols: []engineapi.GenericPolicy{
				engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
							MutateDigest: nil,
							VerifyDigest: nil,
						},
					},
				}),
			},
			expectedCount: 1,
		},
		{
			name: "MutateDigest true -- needs mutation",
			ivpols: []engineapi.GenericPolicy{
				engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
							MutateDigest: &trueVal,
							VerifyDigest: &falseVal,
						},
					},
				}),
			},
			expectedCount: 1,
		},
		{
			// VerifyDigest is a validation concern (the validating webhook asserts the
			// image carries a digest); it never requires a mutating webhook, which can
			// only ever pin digests.
			name: "VerifyDigest true but MutateDigest false -- does not need mutation",
			ivpols: []engineapi.GenericPolicy{
				engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
							MutateDigest: &falseVal,
							VerifyDigest: &trueVal,
						},
					},
				}),
			},
			expectedCount: 0,
		},
		{
			name: "nil MutateDigest defaults to true even when VerifyDigest is false -- needs mutation",
			ivpols: []engineapi.GenericPolicy{
				engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
							MutateDigest: nil,
							VerifyDigest: &falseVal,
						},
					},
				}),
			},
			expectedCount: 1,
		},
		{
			// Core regression: test C from the issue -- both false means no mutating
			// webhook should be registered.
			name: "both false -- does not need mutation",
			ivpols: []engineapi.GenericPolicy{
				engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
							MutateDigest: &falseVal,
							VerifyDigest: &falseVal,
						},
					},
				}),
			},
			expectedCount: 0,
		},
		{
			name: "mixed -- only policies needing mutation are kept",
			ivpols: []engineapi.GenericPolicy{
				engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
							MutateDigest: &trueVal,
							VerifyDigest: &falseVal,
						},
					},
				}),
				engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{
							MutateDigest: &falseVal,
							VerifyDigest: &falseVal,
						},
					},
				}),
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ivpolsNeedingMutation(tt.ivpols)
			assert.Equal(t, tt.expectedCount, len(got))
		})
	}
}
