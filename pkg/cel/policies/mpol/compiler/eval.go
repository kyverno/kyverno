package compiler

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

type EvaluationResult struct {
	PatchedResource unstructured.Unstructured
}
