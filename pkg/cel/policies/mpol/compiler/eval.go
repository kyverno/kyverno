package compiler

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	compiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	admission "k8s.io/apiserver/pkg/admission"
)

type EvaluationResult struct {
	PatchedResource  *unstructured.Unstructured
	Exceptions       []*policiesv1beta1.PolicyException
	AuditAnnotations map[string]string
	Error            error
	// Trace is the decision trace for this evaluation. It is nil unless the policy was compiled
	// with tracing on, so callers must nil-check it. Mirrors vpol's EvaluationResult.Trace
	// (pkg/cel/policies/vpol/compiler/eval.go).
	Trace *trace.Decision
	// Skipped is set when a match condition excluded the resource. Without tracing that case
	// returns a nil result, and it still does; a non-nil skipped result is only returned when
	// tracing is on, so the match traces are not lost. Mirrors vpol's EvaluationResult.Skipped.
	Skipped bool
}

// prepareData assembles the CEL activation data for a single evaluation.
// requestMapFn lazily builds the `request` value: the caller (see
// compiler.BuildNormalizedRequestMap) wraps it once per Handle()/Evaluate()
// call in a memoizing func (for example sync.OnceValues) so it is built at
// most once even though it is threaded into every policy's evaluation - but
// only if some policy actually reaches this function, since matching
// happens before prepareData is ever called. A request matching zero
// policies therefore never pays the request-map build cost. When
// requestMapFn is nil, the map is built locally from request instead,
// self-extracting object/oldObject.
func prepareData(
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace *corev1.Namespace,
	requestMapFn func() (map[string]any, error),
) (map[string]any, error) {
	activation, err := compiler.PrepareK8sActivation(attr, request, namespace, requestMapFn, compiler.BuildNormalizedRequestMap)
	if err != nil {
		return nil, err
	}
	return activation.ToMap(), nil
}
