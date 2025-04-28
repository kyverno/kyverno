package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Policy struct {
	Policy     *policiesv1alpha1.ImageValidatingPolicy
	Exceptions []*policiesv1alpha1.PolicyException
	Actions    sets.Set[admissionregistrationv1.ValidationAction]
}
