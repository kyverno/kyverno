package exceptions

import (
	"errors"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/labels"
)

type fakeLister struct {
	items []*kyvernov2.PolicyException
	err   error
}

func (f fakeLister) List(selector labels.Selector) ([]*kyvernov2.PolicyException, error) {
	return f.items, f.err
}

func TestSelector_Find_FiltersByPolicyAndRule(t *testing.T) {
	pe1 := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{
				PolicyName: "ns1/policyA",
				RuleNames:  []string{"rule1", "rule*"},
			}},
		},
	}
	pe2 := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{
				PolicyName: "ns1/policyB",
				RuleNames:  []string{"rule2"},
			}},
		},
	}
	pe3 := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{
				PolicyName: "ns1/policyA",
				RuleNames:  []string{"other"},
			}},
		},
	}

	s := New(fakeLister{items: []*kyvernov2.PolicyException{pe1, pe2, pe3}})

	// matches wildcard rule*
	res, err := s.Find("ns1/policyA", "rule3")
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Same(t, pe1, res[0])

	// exact match
	res, err = s.Find("ns1/policyB", "rule2")
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Same(t, pe2, res[0])

	// no match
	res, err = s.Find("ns1/policyA", "nomatch")
	assert.NoError(t, err)
	assert.Len(t, res, 0)
}

func TestSelector_Find_PropagatesError(t *testing.T) {
	s := New(fakeLister{err: errors.New("boom")})
	res, err := s.Find("pol", "rule")
	assert.Nil(t, res)
	assert.Error(t, err)
	assert.Equal(t, "boom", err.Error())
}

func TestSelector_Find_EmptyExceptionList(t *testing.T) {
	s := New(fakeLister{items: []*kyvernov2.PolicyException{}})
	res, err := s.Find("any-policy", "any-rule")
	assert.NoError(t, err)
	assert.Len(t, res, 0)
}

func TestSelector_Find_NilExceptionList(t *testing.T) {
	s := New(fakeLister{items: nil})
	res, err := s.Find("any-policy", "any-rule")
	assert.NoError(t, err)
	assert.Len(t, res, 0)
}

func TestSelector_Find_MultipleMatchingExceptions(t *testing.T) {
	// Two exceptions both match the same policy/rule
	pe1 := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{
				PolicyName: "test-policy",
				RuleNames:  []string{"test-rule"},
			}},
		},
	}
	pe2 := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{
				PolicyName: "test-policy",
				RuleNames:  []string{"*"}, // wildcard matches all rules
			}},
		},
	}

	s := New(fakeLister{items: []*kyvernov2.PolicyException{pe1, pe2}})
	res, err := s.Find("test-policy", "test-rule")
	assert.NoError(t, err)
	assert.Len(t, res, 2, "both exceptions should match")
}

func TestSelector_Find_ClusterPolicyWithoutNamespace(t *testing.T) {
	pe := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{
				PolicyName: "cluster-policy", // no namespace prefix
				RuleNames:  []string{"rule1"},
			}},
		},
	}

	s := New(fakeLister{items: []*kyvernov2.PolicyException{pe}})

	// Should match cluster policy without namespace
	res, err := s.Find("cluster-policy", "rule1")
	assert.NoError(t, err)
	assert.Len(t, res, 1)

	// Should NOT match namespaced policy with same name
	res, err = s.Find("default/cluster-policy", "rule1")
	assert.NoError(t, err)
	assert.Len(t, res, 0)
}

func TestSelector_Find_NamespacedPolicyMatching(t *testing.T) {
	pe := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{
				PolicyName: "prod/my-policy",
				RuleNames:  []string{"validate-labels"},
			}},
		},
	}

	s := New(fakeLister{items: []*kyvernov2.PolicyException{pe}})

	// Should match exact namespace/policy
	res, err := s.Find("prod/my-policy", "validate-labels")
	assert.NoError(t, err)
	assert.Len(t, res, 1)

	// Should NOT match different namespace
	res, err = s.Find("dev/my-policy", "validate-labels")
	assert.NoError(t, err)
	assert.Len(t, res, 0)

	// Should NOT match policy without namespace
	res, err = s.Find("my-policy", "validate-labels")
	assert.NoError(t, err)
	assert.Len(t, res, 0)
}

func TestSelector_Find_WildcardRulePatterns(t *testing.T) {
	pe := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{{
				PolicyName: "test-policy",
				RuleNames:  []string{"validate-*", "*-images", "exact-rule"},
			}},
		},
	}

	s := New(fakeLister{items: []*kyvernov2.PolicyException{pe}})

	tests := []struct {
		rule        string
		shouldMatch bool
	}{
		{"validate-labels", true}, // matches validate-*
		{"validate-images", true}, // matches validate-*
		{"check-images", true},    // matches *-images
		{"verify-images", true},   // matches *-images
		{"exact-rule", true},      // exact match
		{"other-rule", false},     // no match
		{"validate", false},       // validate-* needs suffix
	}

	for _, tt := range tests {
		res, err := s.Find("test-policy", tt.rule)
		assert.NoError(t, err)
		if tt.shouldMatch {
			assert.Len(t, res, 1, "rule %q should match", tt.rule)
		} else {
			assert.Len(t, res, 0, "rule %q should not match", tt.rule)
		}
	}
}

func TestSelector_Find_MultipleExceptionsInSinglePolicyException(t *testing.T) {
	// Single PolicyException with multiple Exception entries
	pe := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "policy-a",
					RuleNames:  []string{"rule-a"},
				},
				{
					PolicyName: "policy-b",
					RuleNames:  []string{"rule-b"},
				},
			},
		},
	}

	s := New(fakeLister{items: []*kyvernov2.PolicyException{pe}})

	// Should match policy-a/rule-a
	res, err := s.Find("policy-a", "rule-a")
	assert.NoError(t, err)
	assert.Len(t, res, 1)

	// Should match policy-b/rule-b
	res, err = s.Find("policy-b", "rule-b")
	assert.NoError(t, err)
	assert.Len(t, res, 1)

	// Should NOT match policy-a/rule-b
	res, err = s.Find("policy-a", "rule-b")
	assert.NoError(t, err)
	assert.Len(t, res, 0)
}
