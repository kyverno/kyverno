package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Policy struct {
	Actions        sets.Set[admissionregistrationv1.ValidationAction]
	Policy         policiesv1beta1.ValidatingPolicyLike
	CompiledPolicy *compiler.Policy
}
