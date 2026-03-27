package compiler

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	patch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/cel/mutation/dynamic"

	cel "github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
)

var applyConfigObjectType = celtypes.NewObjectType("Object")

type applyConfigPatcher struct {
	evaluator plugincel.MutatingEvaluator
	prog      cel.Program
}

func newApplyConfigPatcher(eval plugincel.MutatingEvaluator, prog cel.Program) Patcher {
	return &applyConfigPatcher{
		evaluator: eval,
		prog:      prog,
	}
}

func (a *applyConfigPatcher) Patch(ctx context.Context, evalData map[string]any, patchRequest patch.Request, runtimeCELCostBudget int64) (runtime.Object, error) {
	compileErrors := a.evaluator.CompilationErrors()
	if len(compileErrors) > 0 {
		return nil, errors.Join(compileErrors...)
	}
	// can this just be replaced with context eval ?
	// a map string any containing the same stuff the activation
	// contained plus the variables
	out, _, err := a.prog.ContextEval(ctx, map[string]any{})
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
