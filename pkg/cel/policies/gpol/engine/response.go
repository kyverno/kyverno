package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EngineResponse struct {
	Trigger  *unstructured.Unstructured
	Policies []GeneratingPolicyResponse
}

type GeneratingPolicyResponse struct {
	Policy policiesv1beta1.GeneratingPolicyLike
	Result *engineapi.RuleResponse
	// Trace explains how this policy reached its result. It is nil unless the policy was
	// compiled with tracing on (compiler.NewCompilerWithTrace).
	Trace *trace.Decision
}
