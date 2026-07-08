package compiler

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// applyStructuredMergeDiff applies an ApplyConfiguration patch onto the original object
// using structured merge, the same way the upstream apiserver does for
// MutatingAdmissionPolicy (k8s.io/apiserver .../plugin/policy/mutating/patch.ApplyStructuredMergeDiff),
// with one deliberate difference: it does not run the upstream validatePatch guard that
// rejects any atomic array/map/struct present in the patch.
//
// The upstream guard exists to stop a caller from accidentally dropping unset fields when
// they only mean to change part of an atomic value. But it blocks legitimate mutations that
// plain Server-Side Apply already allows, for example adding an init container with args,
// an env var with valueFrom.fieldRef, or a projected volume (issue #15094). Users hit this
// migrating ClusterPolicy mutate rules to MutatingPolicy, and `kubectl apply --server-side`
// applies the exact same configuration without complaint. Kyverno mutations are authored by
// cluster administrators as declarative desired state, so we match SSA semantics here rather
// than the stricter MutatingAdmissionPolicy guard.
func applyStructuredMergeDiff(
	typeConverter managedfields.TypeConverter,
	originalObject runtime.Object,
	patch *unstructured.Unstructured,
) (runtime.Object, error) {
	if patch.GroupVersionKind() != originalObject.GetObjectKind().GroupVersionKind() {
		return nil, fmt.Errorf("patch (%v) and original object (%v) are not of the same gvk", patch.GroupVersionKind().String(), originalObject.GetObjectKind().GroupVersionKind().String())
	} else if typeConverter == nil {
		return nil, fmt.Errorf("type converter must not be nil")
	}

	patchObjTyped, err := typeConverter.ObjectToTyped(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to convert patch object to typed object: %w", err)
	}

	liveObjTyped, err := typeConverter.ObjectToTyped(originalObject, typed.AllowDuplicates)
	if err != nil {
		return nil, fmt.Errorf("failed to convert original object to typed object: %w", err)
	}

	newObjTyped, err := liveObjTyped.Merge(patchObjTyped)
	if err != nil {
		return nil, fmt.Errorf("failed to merge patch: %w", err)
	}

	newObj, err := typeConverter.TypedToObject(newObjTyped)
	if err != nil {
		return nil, fmt.Errorf("failed to convert typed object to object: %w", err)
	}

	return newObj, nil
}
