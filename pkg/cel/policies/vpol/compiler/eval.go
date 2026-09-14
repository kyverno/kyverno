package compiler

import (
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/sdk/extensions/cel/utils"
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
	Exceptions       []*policiesv1beta1.PolicyException
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

// prepareK8sData assembles the CEL activation data for a single evaluation.
// requestMapFn lazily builds the `request` value: the caller (see
// compiler.BuildRawRequestMap) wraps it once per admission request in a
// memoizing func (for example sync.OnceValues) so it is built at most once
// even though it is threaded into every policy's evaluation - but only if
// some policy actually reaches this function, since matching happens before
// prepareK8sData is ever called. A request matching zero policies therefore
// never pays the request-map build cost. When requestMapFn is nil (raw
// payload callers, or the synthetic-request carve-out for ExtractionMode),
// the map is built locally from request instead.
func prepareK8sData(
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	requestMapFn func() (map[string]any, error),
	context libs.Context,
) (evaluationData, error) {
	if attr == nil {
		return evaluationData{}, fmt.Errorf("cannot evaluate Kubernetes-mode policy without admission attributes (hint: use a non-Kubernetes evaluation mode for raw payloads)")
	}
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
	var requestMap map[string]any
	if requestMapFn != nil {
		requestMap, err = requestMapFn()
	} else {
		requestMap, err = compiler.BuildRawRequestMap(request)
	}
	if err != nil {
		return evaluationData{}, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
	}
	return evaluationData{
		Namespace: namespaceVal,
		Object:    objectVal,
		OldObject: oldObjectVal,
		Request:   requestMap,
		Context:   context,
	}, nil
}
