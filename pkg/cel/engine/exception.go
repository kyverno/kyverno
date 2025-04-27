package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

type policyExceptionLister interface {
	List(labels.Selector) ([]*policiesv1alpha1.PolicyException, error)
}

func ListExceptions(lister policyExceptionLister, kind, name string) ([]*policiesv1alpha1.PolicyException, error) {
	exceptions, err := lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var out []*policiesv1alpha1.PolicyException
	for _, exception := range exceptions {
		for _, ref := range exception.Spec.PolicyRefs {
			if ref.Name == name && ref.Kind == kind {
				out = append(out, exception)
			}
		}
	}
	return out, nil
}
