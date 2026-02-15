package patch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v6/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
	"sigs.k8s.io/structured-merge-diff/v6/value"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/managedfields"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	k8spatch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/cel/mutation/dynamic"
)

// NewApplyConfigurationPatcher creates a patcher that performs an applyConfiguration mutation.
// Unlike upstream, this implementation only rejects atomic maps/structs and allows atomic lists.
func NewApplyConfigurationPatcher(expressionEvaluator plugincel.MutatingEvaluator) k8spatch.Patcher {
	return &applyConfigPatcher{expressionEvaluator: expressionEvaluator}
}

type applyConfigPatcher struct {
	expressionEvaluator plugincel.MutatingEvaluator
}

func (e *applyConfigPatcher) Patch(ctx context.Context, r k8spatch.Request, runtimeCELCostBudget int64) (runtime.Object, error) {
	admissionRequest := plugincel.CreateAdmissionRequest(
		r.VersionedAttributes.Attributes,
		metav1.GroupVersionResource(r.MatchedResource),
		metav1.GroupVersionKind(r.VersionedAttributes.VersionedKind),
	)

	compileErrors := e.expressionEvaluator.CompilationErrors()
	if len(compileErrors) > 0 {
		return nil, errors.Join(compileErrors...)
	}
	eval, _, err := e.expressionEvaluator.ForInput(ctx, r.VersionedAttributes, admissionRequest, r.OptionalVariables, r.Namespace, runtimeCELCostBudget)
	if err != nil {
		return nil, err
	}
	if eval.Error != nil {
		return nil, eval.Error
	}
	v := eval.EvalResult

	objVal, ok := v.(*dynamic.ObjectVal)
	if !ok {
		return nil, fmt.Errorf("unsupported return type from ApplyConfiguration expression: %v", v.Type())
	}
	if err := objVal.CheckTypeNamesMatchFieldPathNames(); err != nil {
		return nil, fmt.Errorf("type mismatch: %w", err)
	}

	value, ok := objVal.Value().(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid return type: %T", v)
	}

	patchObject := unstructured.Unstructured{Object: value}
	patchObject.SetGroupVersionKind(r.VersionedAttributes.VersionedObject.GetObjectKind().GroupVersionKind())
	patched, err := applyStructuredMergeDiffAllowAtomicLists(r.TypeConverter, r.VersionedAttributes.VersionedObject, &patchObject)
	if err != nil {
		return nil, fmt.Errorf("error applying patch: %w", err)
	}
	return patched, nil
}

func applyStructuredMergeDiffAllowAtomicLists(
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

	if err := validatePatchDisallowAtomicMapsAndStructs(patchObjTyped); err != nil {
		return nil, fmt.Errorf("invalid ApplyConfiguration: %w", err)
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

func validatePatchDisallowAtomicMapsAndStructs(v *typed.TypedValue) error {
	atomics := findAtomicMapsAndStructs(nil, v.Schema(), v.TypeRef(), v.AsValue())
	if len(atomics) > 0 {
		return fmt.Errorf("may not mutate atomic arrays, maps or structs: %v", strings.Join(atomics, ", "))
	}
	return nil
}

func findAtomicMapsAndStructs(path []fieldpath.PathElement, s *schema.Schema, tr schema.TypeRef, v value.Value) (atomics []string) {
	if a, ok := s.Resolve(tr); ok {
		if v.IsMap() && a.Map != nil {
			if a.Map.ElementRelationship == schema.Atomic {
				atomics = append(atomics, pathString(path))
			}
			v.AsMap().Iterate(func(key string, val value.Value) bool {
				pe := fieldpath.PathElement{FieldName: &key}
				if sf, ok := a.Map.FindField(key); ok {
					tr = sf.Type
					atomics = append(atomics, findAtomicMapsAndStructs(append(path, pe), s, tr, val)...)
				}
				return true
			})
		}
		if v.IsList() && a.List != nil {
			// Intentionally allow atomic lists to support list fields such as
			// container command/args in ApplyConfiguration expressions.
			list := v.AsList()
			for i := 0; i < list.Length(); i++ {
				pe := fieldpath.PathElement{Index: &i}
				atomics = append(atomics, findAtomicMapsAndStructs(append(path, pe), s, a.List.ElementType, list.At(i))...)
			}
		}
	}
	return atomics
}

func pathString(path []fieldpath.PathElement) string {
	sb := strings.Builder{}
	for _, p := range path {
		sb.WriteString(p.String())
	}
	return sb.String()
}
