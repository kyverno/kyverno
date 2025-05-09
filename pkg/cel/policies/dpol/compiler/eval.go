package compiler

import (
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type EvaluationResult struct {
	Error      error
	Message    string
	Result     bool
	Exceptions []*policiesv1alpha1.PolicyException
}

type evaluationData struct {
	Object    any
	Context   libs.Context
	Variables *lazy.MapValue
}

func prepareK8sData(attr admission.Attributes, context libs.Context) (evaluationData, error) {
	objectVal, err := utils.ObjectToResolveVal(attr.GetObject())
	if err != nil {
		return evaluationData{}, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
	}
	return evaluationData{
		Object:  objectVal,
		Context: context,
	}, nil
}
