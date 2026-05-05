package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

type PolicyExceptionLister interface {
	List(labels.Selector) ([]*policiesv1beta1.PolicyException, error)
}

func NewPolicyExceptionLister(lister policiesv1beta1listers.PolicyExceptionLister, namespace string) PolicyExceptionLister {
	if namespace == "" || namespace == "*" {
		return lister
	}

	return lister.PolicyExceptions(namespace)
}

func ListExceptions(lister PolicyExceptionLister, kind, name string) ([]*policiesv1beta1.PolicyException, error) {
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
