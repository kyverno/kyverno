package processor

import (
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/apimachinery/pkg/labels"
)

type policyExceptionLister struct {
	exceptions []*kyvernov2.PolicyException
}

func (l *policyExceptionLister) List(selector labels.Selector) ([]*kyvernov2.PolicyException, error) {
	var out []*kyvernov2.PolicyException
	for _, exception := range l.exceptions {
		exceptionLabels := labels.Set(exception.GetLabels())
		if selector.Matches(exceptionLabels) {
			out = append(out, exception)
		}
	}
	return out, nil
}
