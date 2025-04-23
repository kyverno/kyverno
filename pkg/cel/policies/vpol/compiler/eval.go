package compiler

import (
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type EvaluationResult struct {
	Error            error
	Message          string
	Index            int
	Result           bool
	AuditAnnotations map[string]string
	Exceptions       []*policiesv1alpha1.PolicyException
	PatchedResource  unstructured.Unstructured
}

type evaluationData struct {
	Namespace any
	Object    any
	OldObject any
	Request   any
	Context   libs.Context
	Variables *lazy.MapValue
}

func prepareK8sData(
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
) (evaluationData, error) {
	namespaceVal, err := utils.ObjectToResolveVal(namespace)
	if err != nil {
		return evaluationData{}, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
	}
	objectVal, err := utils.ObjectToResolveVal(attr.GetObject())
	if err != nil {
		return evaluationData{}, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
	}
	oldObjectVal, err := utils.ObjectToResolveVal(attr.GetOldObject())
	if err != nil {
		return evaluationData{}, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
	}
	requestVal, err := utils.ConvertObjectToUnstructured(request)
	if err != nil {
		return evaluationData{}, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
	}
	return evaluationData{
		Namespace: namespaceVal,
		Object:    objectVal,
		OldObject: oldObjectVal,
		Request:   requestVal.Object,
		Context:   context,
	}, nil
}
