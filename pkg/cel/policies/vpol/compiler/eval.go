package compiler

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type EvaluationResult struct {
	Error error
	// MessageExpressionError holds the raw error returned when evaluating the
	// CEL messageExpression field fails at runtime. The Message field is still
	// set to a human-readable fallback string (per the Kubernetes API spec), but
	// this field lets tooling distinguish a legitimate rule failure from one where
	// the failure message itself could not be computed.
	MessageExpressionError error
	Message                string
	Index                  int
	Result                 bool
	AuditAnnotations       map[string]string
	Exceptions             []*policiesv1beta1.PolicyException
	PatchedResource        unstructured.Unstructured
	RefusedException       *RefusedException
	// Trace is the decision trace for this evaluation. It is nil unless the policy was compiled
	// with tracing on, so callers must nil-check it. Only Match, Variables and Verdict are
	// filled here; the policy/resource header and Scope are unknown at this level and are left
	// for the caller to fill in.
	Trace *trace.Decision
	// Skipped is set when a match condition excluded the resource. Without tracing that case
	// returns a nil result, and it still does; a non-nil skipped result is only returned when
	// tracing is on, so the match traces are not lost. Consumers must treat it exactly like nil.
	Skipped bool
}

// RefusedException records an exception that matched but whose compensating controls did not
// hold. It rides alongside the policy's own outcome: a compliant resource needs no exception and
// must not be denied by one, so this is only reported, as the message, once the policy failed.
type RefusedException struct {
	Exception              *policiesv1beta1.PolicyException
	Message                string
	Error                  error
	MessageExpressionError error
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
	activation, err := compiler.PrepareK8sActivation(attr, request, namespace, requestMapFn, compiler.BuildRawRequestMap)
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
