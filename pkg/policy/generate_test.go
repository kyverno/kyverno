package policy

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeClusterPolicy(name string, rules []kyvernov1.Rule) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kyvernov1.Spec{
			Rules: rules,
		},
	}
}

func makeRule(name string, gen *kyvernov1.Generation) kyvernov1.Rule {
	return kyvernov1.Rule{
		Name:       name,
		Generation: gen,
	}
}

func Test_buildPolicyWithDeletedRules(t *testing.T) {
	tests := []struct {
		name         string
		policy       kyvernov1.PolicyInterface
		deletedRules []kyvernov1.Rule
		wantRules    []kyvernov1.Rule
	}{
		{
			name:         "returns policy containing deleted rules",
			policy:       makeClusterPolicy("p", []kyvernov1.Rule{makeRule("rule-a",nil), makeRule("rule-b",nil)}),
			deletedRules: []kyvernov1.Rule{makeRule("rule-a",nil)},
			wantRules:    []kyvernov1.Rule{makeRule("rule-a",nil)},
		},
		{
			name:         "returns policy with empty rules when deletedRules is nil",
			policy:       makeClusterPolicy("p", []kyvernov1.Rule{makeRule("rule-a",nil), makeRule("rule-b",nil)}),
			deletedRules: nil,
			wantRules:    nil,
		},
		{
			name:         "original policy is not mutated",
			policy:       makeClusterPolicy("p", []kyvernov1.Rule{makeRule("rule-a",nil)}),
			deletedRules: []kyvernov1.Rule{makeRule("rule-b",nil)},
			wantRules:    []kyvernov1.Rule{makeRule("rule-b",nil)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		    originalRuleCount := len(tt.policy.GetSpec().Rules)
			
			returnedPolicy := buildPolicyWithDeletedRules(tt.policy, tt.deletedRules)

			if len(returnedPolicy.GetSpec().Rules) != len(tt.wantRules) {
				t.Errorf("buildPolicyWithDeletedRules() rules len = %d, want %d",
				 len(returnedPolicy.GetSpec().Rules), len(tt.wantRules))
			}

			if len(tt.policy.GetSpec().Rules) != originalRuleCount {
				t.Errorf("buildPolicyWithDeletedRules() mutated the orginal policy: had %d rules, now %d",
				 originalRuleCount, len(tt.policy.GetSpec().Rules))
			}
		})
	}
}