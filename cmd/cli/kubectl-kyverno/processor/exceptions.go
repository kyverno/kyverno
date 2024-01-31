package processor

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type policyExceptionLister struct {
	exceptions []kyvernov2beta1.PolicyException
}

func (l *policyExceptionLister) List(selector labels.Selector) ([]*kyvernov2beta1.PolicyException, error) {
	var matchedExceptions []*kyvernov2beta1.PolicyException
	for i := range l.exceptions {
		exceptionLabels := labels.Set(l.exceptions[i].GetLabels())
		if selector.Matches(exceptionLabels) {
			matchedExceptions = append(matchedExceptions, &l.exceptions[i])
		}
	}
	return matchedExceptions, nil
}
