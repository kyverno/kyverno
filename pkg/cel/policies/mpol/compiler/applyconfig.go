package compiler

import (
	"context"
	"errors"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	patch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/mutation/dynamic"
)

type applyConfigPatcher struct {
	evaluator plugincel.MutatingEvaluator
}

func newApplyConfigPatcher(eval plugincel.MutatingEvaluator) Patcher {
	return &applyConfigPatcher{
		evaluator: eval,
	}
}

func (a *applyConfigPatcher) Patch(ctx context.Context, request *admissionv1.AdmissionRequest, patchRequest patch.Request, runtimeCELCostBudget int64) (runtime.Object, error) {
	// panic on nil request?
	compileErrors := a.evaluator.CompilationErrors()
	if len(compileErrors) > 0 {
		return nil, errors.Join(compileErrors...)
	}
	eval, _, err := a.evaluator.ForInput(ctx, patchRequest.VersionedAttributes, request, patchRequest.OptionalVariables, patchRequest.Namespace, celconfig.RuntimeCELCostBudget)
	if err != nil {
		return nil, err
	}
	if eval.Error != nil {
		return nil, eval.Error
	}
	v := eval.EvalResult

	// The compiler ensures that the return type is an ObjectVal with type name of "Object".
	objVal, ok := v.(*dynamic.ObjectVal)
	if !ok {
		// Should not happen since the compiler type checks the return type.
		return nil, fmt.Errorf("unsupported return type from ApplyConfiguration expression: %v", v.Type())
	}

	err = objVal.CheckTypeNamesMatchFieldPathNames()
	if err != nil {
		return nil, fmt.Errorf("type mismatch: %w", err)
	}

	value, ok := objVal.Value().(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid return type: %T", v)
	}

	patchObject := unstructured.Unstructured{Object: value}
	patchObject.SetGroupVersionKind(patchRequest.VersionedAttributes.VersionedObject.GetObjectKind().GroupVersionKind())
	patched, err := patch.ApplyStructuredMergeDiff(patchRequest.TypeConverter, patchRequest.VersionedAttributes.VersionedObject, &patchObject)
	if err != nil {
		return nil, fmt.Errorf("error applying patch: %w", err)
	}

	return patched, nil
}
