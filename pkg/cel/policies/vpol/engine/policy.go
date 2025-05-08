package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Policy struct {
	Actions        sets.Set[admissionregistrationv1.ValidationAction]
	Policy         policiesv1alpha1.ValidatingPolicy
	CompiledPolicy *compiler.Policy
}
