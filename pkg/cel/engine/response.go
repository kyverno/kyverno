package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

type EngineResponse struct {
	Resource *unstructured.Unstructured
	Policies []ValidatingPolicyResponse
}

type ValidatingPolicyResponse struct {
	Actions sets.Set[admissionregistrationv1.ValidationAction]
	Policy  policiesv1beta1.ValidatingPolicyLike
	Rules   []engineapi.RuleResponse
}
