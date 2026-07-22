package compiler

import (
	"context"
	"fmt"

	cel "github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	patch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/cel/mutation/dynamic"
)

var applyConfigObjectType = celtypes.NewObjectType("Object")

type applyConfigPatcher struct {
	prog cel.Program
}

func newApplyConfigPatcher(prog cel.Program) Patcher {
	return &applyConfigPatcher{
		prog: prog,
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

	patchObject := unstructured.Unstructured{Object: value}
	patchObject.SetGroupVersionKind(patchRequest.VersionedAttributes.VersionedObject.GetObjectKind().GroupVersionKind())
	patched, err := patch.ApplyStructuredMergeDiff(patchRequest.TypeConverter, patchRequest.VersionedAttributes.VersionedObject, &patchObject)
	if err != nil {
		return nil, fmt.Errorf("error applying patch: %w", err)
	}

	return patched, nil
}
