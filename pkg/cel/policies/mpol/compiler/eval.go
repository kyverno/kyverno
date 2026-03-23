package compiler

import (
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	compiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/sdk/cel/utils"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	admission "k8s.io/apiserver/pkg/admission"
)

type EvaluationResult struct {
	PatchedResource *unstructured.Unstructured
	Exceptions      []*policiesv1beta1.PolicyException
	Error           error
}

func prepareData(
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace *corev1.Namespace,
	context libs.Context,
) (map[string]any, error) {
	if attr == nil {
		return nil, fmt.Errorf("cannot evaluate Kubernetes-mode policy without admission attributes (hint: use a non-Kubernetes evaluation mode for raw payloads)")
	}
	namespaceVal, err := utils.ObjectToResolveVal(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
	}
	objectVal, err := utils.ObjectToResolveVal(attr.GetObject())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
	}
	oldObjectVal, err := utils.ObjectToResolveVal(attr.GetOldObject())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
	}

	var requestVal map[string]any
	if request != nil {
		object, oldObject, err := admissionutils.ExtractResources(nil, *request)
		if err != nil {
			return nil, fmt.Errorf("failed to extract resources from admission request: %w", err)
		}

		result, err := utils.ConvertObjectToUnstructured(request)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
		} else if result != nil {
			requestVal = result.Object
			requestVal["object"] = object.Object
			requestVal["oldObject"] = oldObject.Object
		}
	}
	return map[string]any{
		compiler.NamespaceObjectKey: namespaceVal,
		compiler.ObjectKey:          objectVal,
		compiler.OldObjectKey:       oldObjectVal,
		compiler.RequestKey:         requestVal,
	}, nil
}
