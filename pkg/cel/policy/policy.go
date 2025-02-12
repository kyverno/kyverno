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
	Error   error
	Message string
	Result  ref.Val
}

type CompiledPolicy interface {
	Evaluate(context.Context, admission.Attributes, *admissionv1.AdmissionRequest, runtime.Object, contextlib.ContextInterface) ([]EvaluationResult, error)
}

type compiledValidation struct {
	message           string
	messageExpression cel.Program
	program           cel.Program
}

type compiledPolicy struct {
	failurePolicy        admissionregistrationv1.FailurePolicyType
	matchConditions      []cel.Program
	variables            map[string]cel.Program
	validations          []compiledValidation
	auditAnnotations     map[string]cel.Program
	polexMatchConditions []cel.Program
}

func (p *compiledPolicy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context contextlib.ContextInterface,
) ([]EvaluationResult, error) {
	// check if the resource matches an exception
	if len(p.polexMatchConditions) > 0 {
		match, err := p.match(ctx, attr, request, namespace, true)
		if err != nil {
			return nil, err
		}
		if match {
			return nil, nil
		}
	}

	match, err := p.match(ctx, attr, request, namespace, false)
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
			out, _, err := variable.ContextEval(ctx, data)
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
	for _, validation := range p.validations {
		out, _, err := validation.program.ContextEval(ctx, data)
		// evaluate only when rule fails
		var message string
		if outcome, err := utils.ConvertToNative[bool](out); err == nil && !outcome {
			message = validation.message
			if validation.messageExpression != nil {
				if out, _, err := validation.messageExpression.ContextEval(ctx, data); err != nil {
					message = fmt.Sprintf("failed to evaluate message expression: %s", err)
				} else if msg, err := utils.ConvertToNative[string](out); err != nil {
					message = fmt.Sprintf("failed to convert message expression to string: %s", err)
				} else {
					message = msg
				}
			}
		}
		results = append(results, EvaluationResult{
			Result:  out,
			Message: message,
			Error:   err,
		})
	}
	return results, nil
}

func (p *compiledPolicy) match(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	isException bool,
) (bool, error) {
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

	matchConditions := p.matchConditions
	if isException {
		matchConditions = p.polexMatchConditions
	}
	for _, matchCondition := range matchConditions {
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
	if err := multierr.Combine(errs...); err == nil {
		return true, nil
	} else if p.failurePolicy == admissionregistrationv1.Ignore {
		return false, nil
	} else {
		return false, err
	}
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
