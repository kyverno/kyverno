package policy

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type EvaluationResult struct {
	Result ref.Val
	Error  error
}

type CompiledPolicy interface {
	Evaluate(context.Context, admission.Attributes, *admissionv1.AdmissionRequest, runtime.Object, contextlib.ContextInterface) ([]EvaluationResult, error)
}

type compiledPolicy struct {
	failurePolicy    admissionregistrationv1.FailurePolicyType
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	validations      []cel.Program
	auditAnnotations map[string]cel.Program
}

func (p *compiledPolicy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context contextlib.ContextInterface,
) ([]EvaluationResult, error) {
	match, err := p.match(ctx, attr, request, namespace)
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, nil
	}
	namespaceVal, err := objectToResolveVal(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
	}
	objectVal, err := objectToResolveVal(attr.GetObject())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
	}
	oldObjectVal, err := objectToResolveVal(attr.GetOldObject())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
	}
	requestVal, err := convertObjectToUnstructured(request)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
	}
	vars := lazy.NewMapValue(VariablesType)
	data := map[string]any{
		ContextKey:         contextlib.Context{ContextInterface: context},
		NamespaceObjectKey: namespaceVal,
		ObjectKey:          objectVal,
		OldObjectKey:       oldObjectVal,
		RequestKey:         requestVal.Object,
		VariablesKey:       vars,
	}
	for name, variable := range p.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.Eval(data)
			if out != nil {
				return out
			}
			if err != nil {
				return types.WrapErr(err)
			}
			return nil
		})
	}
	results := make([]EvaluationResult, 0, len(p.validations))
	for _, rule := range p.validations {
		out, _, err := rule.Eval(data)
		results = append(results, EvaluationResult{
			Result: out,
			Error:  err,
		})
	}
	return results, nil
}

func (p *compiledPolicy) match(ctx context.Context, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object) (bool, error) {
	namespaceVal, err := objectToResolveVal(namespace)
	if err != nil {
		return false, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
	}
	objectVal, err := objectToResolveVal(attr.GetObject())
	if err != nil {
		return false, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
	}
	oldObjectVal, err := objectToResolveVal(attr.GetOldObject())
	if err != nil {
		return false, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
	}
	requestVal, err := convertObjectToUnstructured(request)
	if err != nil {
		return false, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
	}
	data := map[string]any{
		NamespaceObjectKey: namespaceVal,
		ObjectKey:          objectVal,
		OldObjectKey:       oldObjectVal,
		RequestKey:         requestVal.Object,
	}
	var errs []error
	for _, matchCondition := range p.matchConditions {
		// evaluate the condition
		out, _, err := matchCondition.ContextEval(ctx, data)
		// check error
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// try to convert to a bool
		result, err := utils.ConvertToNative[bool](out)
		// check error
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// if condition is false, skip
		if !result {
			return false, nil
		}
	}
	return true, multierr.Combine(errs...)
}

func convertObjectToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return &unstructured.Unstructured{Object: nil}, nil
	}
	ret, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: ret}, nil
}

func objectToResolveVal(r runtime.Object) (interface{}, error) {
	if r == nil || reflect.ValueOf(r).IsNil() {
		return nil, nil
	}
	v, err := convertObjectToUnstructured(r)
	if err != nil {
		return nil, err
	}
	return v.Object, nil
}
