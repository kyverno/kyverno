package engine

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EngineResponse struct {
	Trigger  *unstructured.Unstructured
	Policies []GeneratingPolicyResponse
}

type GeneratingPolicyResponse struct {
	Policy policiesv1alpha1.GeneratingPolicy
	Result *engineapi.RuleResponse
}
