package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
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
	Policy  policiesv1alpha1.ValidatingPolicy
	Rules   []engineapi.RuleResponse
}
