package compiler

import (
	"context"
	"fmt"

	cel "github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	patch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/cel/mutation/dynamic"
)

var applyConfigObjectType = celtypes.NewObjectType("Object")

type applyConfigPatcher struct {
	prog cel.Program
	// useServerSideApply routes the merge through the local guard-free implementation when set,
	// allowing atomic fields to be mutated. When false the upstream guarded implementation is used.
	useServerSideApply bool
}

func newApplyConfigPatcher(prog cel.Program, useServerSideApply bool) Patcher {
	return &applyConfigPatcher{
		prog:               prog,
		useServerSideApply: useServerSideApply,
	}
}

func (a *applyConfigPatcher) Patch(ctx context.Context, evalData map[string]any, patchRequest patch.Request, runtimeCELCostBudget int64) (runtime.Object, error) {
	out, _, err := a.prog.ContextEval(ctx, evalData)
	if err != nil {
		return nil, err
	}

	// The compiler ensures that the return type is an ObjectVal with type name of "Object".
	objVal, ok := out.(*dynamic.ObjectVal)
	if !ok {
		// Should not happen since the compiler type checks the return type.
		return nil, fmt.Errorf("unsupported return type from ApplyConfiguration expression: %v", out.Type())
	}

	err = objVal.CheckTypeNamesMatchFieldPathNames()
	if err != nil {
		return nil, fmt.Errorf("type mismatch: %w", err)
	}

	value, ok := objVal.Value().(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid return type: %T", out)
	}

	normalized, err := normalizeApplyConfigValue(value)
	if err != nil {
		return nil, fmt.Errorf("invalid patch value: %w", err)
	}

	patchObject := unstructured.Unstructured{Object: normalized.(map[string]any)}
	patchObject.SetGroupVersionKind(patchRequest.VersionedAttributes.VersionedObject.GetObjectKind().GroupVersionKind())

	mergeFn := patch.ApplyStructuredMergeDiff
	if a.useServerSideApply {
		mergeFn = applyStructuredMergeDiff
	}
	patched, err := mergeFn(patchRequest.TypeConverter, patchRequest.VersionedAttributes.VersionedObject, &patchObject)
	if err != nil {
		return nil, fmt.Errorf("error applying patch: %w", err)
	}

	return patched, nil
}

// normalizeApplyConfigValue converts leftover CEL values into plain Go values.
// Upstream conversion only calls Value() on list elements, so a bare map literal
// like [{"name": "x"}] stays a CEL map that structured-merge-diff cannot read.
// See https://github.com/kyverno/kyverno/issues/17734.
func normalizeApplyConfigValue(value any) (any, error) {
	switch v := value.(type) {
	case ref.Val:
		return normalizeApplyConfigValue(v.Value())
	case *map[string]any:
		if v == nil {
			return nil, nil
		}
		return normalizeApplyConfigValue(*v)
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, item := range v {
			normalized, err := normalizeApplyConfigValue(item)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case map[ref.Val]ref.Val:
		result := make(map[string]any, len(v))
		for key, item := range v {
			stringKey, ok := key.Value().(string)
			if !ok {
				return nil, fmt.Errorf("map key %q is of type %T, not string", key, key)
			}
			normalized, err := normalizeApplyConfigValue(item)
			if err != nil {
				return nil, err
			}
			result[stringKey] = normalized
		}
		return result, nil
	case []any:
		result := make([]any, 0, len(v))
		for _, item := range v {
			normalized, err := normalizeApplyConfigValue(item)
			if err != nil {
				return nil, err
			}
			result = append(result, normalized)
		}
		return result, nil
	case []ref.Val:
		result := make([]any, 0, len(v))
		for _, item := range v {
			normalized, err := normalizeApplyConfigValue(item)
			if err != nil {
				return nil, err
			}
			result = append(result, normalized)
		}
		return result, nil
	default:
		return value, nil
	}
}
