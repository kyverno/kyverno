package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

func ListExceptions(polexLister policiesv1alpha1listers.PolicyExceptionLister, policyName, policyKind string) ([]*policiesv1alpha1.PolicyException, error) {
	polexList, err := polexLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var exceptions []*policiesv1alpha1.PolicyException
	for _, polex := range polexList {
		for _, ref := range polex.Spec.PolicyRefs {
			if ref.Name == policyName && ref.Kind == policyKind {
				exceptions = append(exceptions, polex)
			}
		}
	}
	return exceptions, nil
}
