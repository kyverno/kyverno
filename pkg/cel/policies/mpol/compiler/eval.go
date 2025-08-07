package compiler

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EvaluationResult struct {
	PatchedResource *unstructured.Unstructured
	Exceptions      []*policiesv1alpha1.PolicyException
	Error           error
}
