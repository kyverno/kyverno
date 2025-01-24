package exceptions

import (
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/apimachinery/pkg/labels"
)

type Lister interface {
	List(labels.Selector) ([]*kyvernov2.PolicyException, error)
}

type selector struct {
	lister Lister
}

func New(lister Lister) selector {
	return selector{
		lister: lister,
	}
}

func (s selector) Find(policyName string, ruleName string) ([]*kyvernov2.PolicyException, error) {
	polexs, err := s.lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var results []*kyvernov2.PolicyException
	for _, polex := range polexs {
		if polex.Contains(policyName, ruleName) {
			results = append(results, polex)
		}
	}
	return results, nil
}
