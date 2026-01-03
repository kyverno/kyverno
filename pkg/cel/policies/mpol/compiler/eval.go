package compiler

import (
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EvaluationResult struct {
	PatchedResource *unstructured.Unstructured
	Exceptions      []*policiesv1beta1.PolicyException
	Error           error
}
