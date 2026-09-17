package compiler

import (
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	compiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/sdk/extensions/cel/utils"
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

	var requestMap map[string]any
	if requestMapFn != nil {
		requestMap, err = requestMapFn()
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
		}
	} else if request != nil {
		requestMap, err = compiler.BuildNormalizedRequestMap(request)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
		}
	}
	return map[string]any{
		compiler.NamespaceObjectKey: namespaceVal,
		compiler.ObjectKey:          objectVal,
		compiler.OldObjectKey:       oldObjectVal,
		compiler.RequestKey:         requestMap,
	}, nil
}
