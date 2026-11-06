package compiler

import (
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type evaluationData struct {
	Namespace any
	Object    any
	OldObject any
	Request   any
	Context   libs.Context
	Variables *lazy.MapValue
}

func prepareData(
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
) (evaluationData, error) {
	activation, err := compiler.PrepareK8sActivation(attr, request, namespace, nil, compiler.BuildRawRequestMap)
	if err != nil {
		return evaluationData{}, err
	}
	return evaluationData{
		Namespace: activation.Namespace,
		Object:    activation.Object,
		OldObject: activation.OldObject,
		Request:   activation.Request,
		Context:   context,
	}, nil
}
