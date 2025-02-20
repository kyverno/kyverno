package policy

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
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
	Error            error
	Message          string
	Index            int
	Result           bool
	AuditAnnotations map[string]string
	Exceptions       []policiesv1alpha1.CELPolicyException
}

type CompiledPolicy interface {
	Evaluate(context.Context, admission.Attributes, *admissionv1.AdmissionRequest, runtime.Object, contextlib.ContextInterface, int) (*EvaluationResult, error)
}

type compiledValidation struct {
	message           string
	messageExpression cel.Program
	program           cel.Program
}

type compiledAutogenRule struct {
	matchConditions []cel.Program
	validations     []compiledValidation
	auditAnnotation map[string]cel.Program
	variables       map[string]cel.Program
}

type compiledException struct {
	exception       policiesv1alpha1.CELPolicyException
	matchConditions []cel.Program
}

type compiledPolicy struct {
	failurePolicy    admissionregistrationv1.FailurePolicyType
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	validations      []compiledValidation
	auditAnnotations map[string]cel.Program
	autogenRules     []compiledAutogenRule
	exceptions       []compiledException
}

func (p *compiledPolicy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context contextlib.ContextInterface,
	autogenIndex int,
) (*EvaluationResult, error) {
	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]policiesv1alpha1.CELPolicyException, 0)
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, attr, request, namespace, polex.matchConditions)
			if err != nil {
				return nil, err
			}
			if match {
				matchedExceptions = append(matchedExceptions, polex.exception)
			}
		}
		if len(matchedExceptions) > 0 {
			return &EvaluationResult{Exceptions: matchedExceptions}, nil
		}
	}

	var matchConditions []cel.Program
	var validations []compiledValidation
	var variables map[string]cel.Program

	if autogenIndex != -1 {
		matchConditions = p.autogenRules[autogenIndex].matchConditions
		validations = p.autogenRules[autogenIndex].validations
		variables = p.autogenRules[autogenIndex].variables
	} else {
		matchConditions = p.matchConditions
		validations = p.validations
		variables = p.variables
	}
	match, err := p.match(ctx, attr, request, namespace, matchConditions)
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
	for name, variable := range variables {
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

	for index, validation := range validations {
		out, _, err := validation.program.ContextEval(ctx, data)
		if err != nil {
			return nil, err
		}

		// evaluate only when rule fails
		if outcome, err := utils.ConvertToNative[bool](out); err == nil && !outcome {
			message := validation.message
			if validation.messageExpression != nil {
				if out, _, err := validation.messageExpression.ContextEval(ctx, data); err != nil {
					message = fmt.Sprintf("failed to evaluate message expression: %s", err)
				} else if msg, err := utils.ConvertToNative[string](out); err != nil {
					message = fmt.Sprintf("failed to convert message expression to string: %s", err)
				} else {
					message = msg
				}
			}

			auditAnnotations := make(map[string]string, 0)
			for key, annotation := range p.auditAnnotations {
				out, _, err := annotation.ContextEval(ctx, data)
				if err != nil {
					return nil, fmt.Errorf("failed to evaluate auditAnnotation '%s': %w", key, err)
				}
				// evaluate only when rule fails
				if outcome, err := utils.ConvertToNative[string](out); err == nil && outcome != "" {
					auditAnnotations[key] = outcome
				} else if err != nil {
					return nil, fmt.Errorf("failed to convert auditAnnotation '%s' expression: %w", key, err)
				}
			}

			return &EvaluationResult{
				Result:           outcome,
				Message:          message,
				Index:            index,
				Error:            err,
				AuditAnnotations: auditAnnotations,
			}, nil
		} else if err != nil {
			return &EvaluationResult{Error: err}, nil
		}
	}

	return &EvaluationResult{Result: true}, nil
}

func (p *compiledPolicy) match(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	matchConditions []cel.Program,
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
