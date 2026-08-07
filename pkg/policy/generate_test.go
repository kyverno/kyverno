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
			originalRules := make([]string, len(tt.policy.GetSpec().Rules))

			for i, r := range tt.policy.GetSpec().Rules {
				originalRules[i] = r.Name	
			}
			
			returnedPolicy := buildPolicyWithDeletedRules(tt.policy, tt.deletedRules)
            
			returnedRules := returnedPolicy.GetSpec().Rules
			if len(returnedRules) != len(tt.wantRules) {
				t.Errorf("buildPolicyWithDeletedRules() rules len = %d, want %d",
				 len(returnedPolicy.GetSpec().Rules), len(tt.wantRules))
			}
            
            for i, wantRule := range tt.wantRules {
				if i >= len(returnedRules) {
					break
				}
				if returnedRules[i].Name != wantRule.Name {
					t.Errorf("buildPolicyWithDeletedRules() rule[%d].Name = %q, want = %q",
					 i, returnedRules[i].Name, wantRule.Name)
				}
			}
            
			currentRules := tt.policy.GetSpec().Rules
			if len(currentRules) != len(originalRules) {
				t.Errorf("buildPolicyWithDeletedRules() changed original policy rule count : was %d, now %d",
				 len(originalRules), len(currentRules))
			}

			for i, originalRule := range originalRules {
				if i >= len(originalRules) {
					break
				}

				if currentRules[i].Name != originalRule {
					t.Errorf("buildPolicyWithDeletedRules() mutated original policy rule[%d]: was %q, now %q",
					 i, originalRule, currentRules[i].Name)
				}
			}
		})
	}
}