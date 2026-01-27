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
