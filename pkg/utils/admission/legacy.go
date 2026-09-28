package admission

import (
	"encoding/json"

	admissionv1 "k8s.io/api/admission/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// IsFinalizerRemovalOnTerminatingObject reports whether an UPDATE request removes one or more
// finalizers from an already-terminating object and changes nothing else, the one narrow
// recovery path that lets a finalizer stuck on a legacy kyverno.io policy kind be cleared once
// no other write to that object is allowed. A request that fails any of the checks below, or
// that cannot be decoded, is not a finalizer-removal request and the caller must deny it; a
// decode error is therefore reported as (false, err), never as (false, nil).
func IsFinalizerRemovalOnTerminatingObject(request admissionv1.AdmissionRequest) (bool, error) {
	if request.Operation != admissionv1.Update || request.SubResource != "" {
		return false, nil
	}

	old, err := UnmarshalPartialObjectMetadata(request.OldObject.Raw)
	if err != nil {
		return false, err
	}
	new, err := UnmarshalPartialObjectMetadata(request.Object.Raw)
	if err != nil {
		return false, err
	}

	// Clients cannot set deletionTimestamp themselves; only a real DELETE produces it, so
	// this precondition makes the carve-out unforgeable without first going through the
	// already-allowed DELETE path.
	if old.DeletionTimestamp == nil {
		return false, nil
	}
	// The apiserver guarantees this is unchanged on update; check anyway so a malformed
	// request denies rather than passes.
	if !old.DeletionTimestamp.Equal(new.DeletionTimestamp) {
		return false, nil
	}

	oldFinalizers := sets.New(old.Finalizers...)
	newFinalizers := sets.New(new.Finalizers...)
	if !oldFinalizers.IsSuperset(newFinalizers) {
		return false, nil
	}
	if newFinalizers.Len() >= oldFinalizers.Len() {
		return false, nil
	}

	// Everything else in metadata must be unchanged. Finalizers already compared above;
	// ResourceVersion is excluded because a client may send a stale or empty one;
	// ManagedFields is excluded because the apiserver's field manager rewrites it before
	// admission runs. Nothing else is normalised: labels, annotations, ownerReferences,
	// generation, name, namespace, uid, and creationTimestamp must match exactly.
	oldMeta := normalizeForFinalizerCompare(old)
	newMeta := normalizeForFinalizerCompare(new)
	if !apiequality.Semantic.DeepEqual(oldMeta, newMeta) {
		return false, nil
	}

	// Everything outside metadata (spec, status, and any unknown top-level key) must be
	// byte-for-byte equivalent, compared as opaque JSON so this check has no typed
	// dependency on any policy kind.
	oldBody, err := bodyWithoutMetadata(request.OldObject.Raw)
	if err != nil {
		return false, err
	}
	newBody, err := bodyWithoutMetadata(request.Object.Raw)
	if err != nil {
		return false, err
	}
	if !apiequality.Semantic.DeepEqual(oldBody, newBody) {
		return false, nil
	}

	return true, nil
}

func normalizeForFinalizerCompare(object *metav1.PartialObjectMetadata) metav1.ObjectMeta {
	meta := object.ObjectMeta.DeepCopy()
	meta.Finalizers = nil
	meta.ResourceVersion = ""
	meta.ManagedFields = nil
	return *meta
}

func bodyWithoutMetadata(raw []byte) (map[string]interface{}, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	delete(doc, "metadata")
	return doc, nil
}
