package compiler

import (
	"fmt"

	"github.com/kyverno/sdk/extensions/cel/utils"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
)

// RequestMapBuilder builds a CEL `request` activation map from an admission request.
type RequestMapBuilder func(*admissionv1.AdmissionRequest) (map[string]any, error)

// ActivationData holds the standard resolved Kubernetes admission activation values
// ready for CEL evaluation.
type ActivationData struct {
	Namespace any
	Object    any
	OldObject any
	Request   map[string]any
}

// ToMap converts ActivationData into a map activation keyed by the canonical
// compiler keys (NamespaceObjectKey, ObjectKey, OldObjectKey, RequestKey).
func (a ActivationData) ToMap() map[string]any {
	return map[string]any{
		NamespaceObjectKey: a.Namespace,
		ObjectKey:          a.Object,
		OldObjectKey:       a.OldObject,
		RequestKey:         a.Request,
	}
}

// PrepareK8sActivation assembles the standard Kubernetes admission activation data
// (namespace, object, oldObject, request) for CEL policy evaluation.
//
// requestMapFn is an optional memoized request-map builder shared across policies in
// the same admission request (e.g. sync.OnceValues). When non-nil, it is called to
// lazily produce the request map. When nil, the fallback builder is called with request.
//
// Each calling engine must explicitly pass its own fallback builder (for example
// BuildRawRequestMap for vpol/gpol/ivpol, or BuildNormalizedRequestMap for mpol) to
// preserve its engine-specific request map semantics.
func PrepareK8sActivation(
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	requestMapFn func() (map[string]any, error),
	fallback RequestMapBuilder,
) (ActivationData, error) {
	if attr == nil {
		return ActivationData{}, fmt.Errorf("cannot evaluate Kubernetes-mode policy without admission attributes (hint: use a non-Kubernetes evaluation mode for raw payloads)")
	}
	if fallback == nil {
		return ActivationData{}, fmt.Errorf("request map builder must not be nil")
	}
	namespaceVal, err := utils.ObjectToResolveVal(namespace)
	if err != nil {
		return ActivationData{}, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
	}
	objectVal, err := utils.ObjectToResolveVal(attr.GetObject())
	if err != nil {
		return ActivationData{}, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
	}
	oldObjectVal, err := utils.ObjectToResolveVal(attr.GetOldObject())
	if err != nil {
		return ActivationData{}, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
	}
	var requestMap map[string]any
	if requestMapFn != nil {
		requestMap, err = requestMapFn()
	} else {
		requestMap, err = fallback(request)
	}
	if err != nil {
		return ActivationData{}, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
	}
	return ActivationData{
		Namespace: namespaceVal,
		Object:    objectVal,
		OldObject: oldObjectVal,
		Request:   requestMap,
	}, nil
}
