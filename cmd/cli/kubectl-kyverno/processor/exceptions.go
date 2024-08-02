package processor

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type policyExceptionLister struct {
	exceptions []*kyvernov2beta1.PolicyException
}

func (l *policyExceptionLister) List(selector labels.Selector) ([]*kyvernov2beta1.PolicyException, error) {
	var out []*kyvernov2beta1.PolicyException
	for _, exception := range l.exceptions {
		exceptionLabels := labels.Set(exception.GetLabels())
		if selector.Matches(exceptionLabels) {
			out = append(out, exception)
		}
	}
	return out, nil
}
