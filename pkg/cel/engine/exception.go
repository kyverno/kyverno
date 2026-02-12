package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type policyExceptionLister interface {
	List(labels.Selector) ([]*policiesv1beta1.PolicyException, error)
}

func ListExceptions(lister policyExceptionLister, kind, name string) ([]*policiesv1beta1.PolicyException, error) {
	exceptions, err := lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var out []*policiesv1beta1.PolicyException
	for _, exception := range exceptions {
		for _, ref := range exception.Spec.PolicyRefs {
			if ref.Name == name && ref.Kind == kind {
				out = append(out, exception)
			}
		}
	}
	return out, nil
}
