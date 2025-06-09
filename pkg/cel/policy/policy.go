package policy

import (
	"context"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
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

type ContextInterface interface {
	globalcontext.ContextInterface
	imagedata.ContextInterface
	resource.ContextInterface
}

type CompiledPolicy interface {
	Evaluate(context.Context, any, admission.Attributes, *admissionv1.AdmissionRequest, runtime.Object, ContextInterface, *string) (*EvaluationResult, error)
}

type EvaluationData struct {
	Namespace any
	Object    any
	OldObject any
	Request   any
	Context   ContextInterface
	Variables *lazy.MapValue
}
