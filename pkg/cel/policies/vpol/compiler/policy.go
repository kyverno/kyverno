package compiler

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/sdk/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	mode             policiesv1beta1.EvaluationMode
	failurePolicy    admissionregistrationv1.FailurePolicyType
	matchConstraints *admissionregistrationv1.MatchResources
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	validations      []compiler.Validation
	auditAnnotations map[string]cel.Program
	exceptions       []compiler.Exception
}

func (p *Policy) MatchConstraints() *admissionregistrationv1.MatchResources {
	return p.matchConstraints
}

func (p *Policy) Evaluate(
	ctx context.Context,
	json any,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
) (*EvaluationResult, error) {
	switch p.mode {
	case policieskyvernoio.EvaluationModeJSON:
		return p.evaluateJson(ctx, json)
	default:
		return p.evaluateKubernetes(ctx, attr, request, namespace, context)
	}
}

func (p *Policy) evaluateJson(
	ctx context.Context,
	json any,
) (*EvaluationResult, error) {
	data := evaluationData{
		Object:    json,
		Variables: lazy.NewMapValue(compiler.VariablesType),
	}
	return p.evaluateWithData(ctx, data)
}

func (p *Policy) evaluateKubernetes(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
) (*EvaluationResult, error) {
	data, err := prepareK8sData(attr, request, namespace, context)
	if err != nil {
		return nil, err
	}
	return p.evaluateWithData(ctx, data)
}

func (p *Policy) evaluateWithData(
	ctx context.Context,
	data evaluationData,
) (*EvaluationResult, error) {
	allowedImages := make([]string, 0)
	allowedValues := make([]string, 0)
	dataNew := map[string]any{
		compiler.NamespaceObjectKey: data.Namespace,
		compiler.ObjectKey:          data.Object,
		compiler.OldObjectKey:       data.OldObject,
		compiler.RequestKey:         data.Request,
	}
	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1beta1.PolicyException, 0)
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, dataNew, polex.MatchConditions)
			if err != nil {
				return nil, err
			}
			if match {
				matchedExceptions = append(matchedExceptions, polex.Exception)
				allowedImages = append(allowedImages, polex.Exception.Spec.Images...)
				allowedValues = append(allowedValues, polex.Exception.Spec.AllowedValues...)
			}
		}
		// if there are matched exceptions and no allowed images, no need to evaluate the policy
		// as the resource is excluded from policy evaluation
		if len(matchedExceptions) > 0 && len(allowedImages) == 0 && len(allowedValues) == 0 {
			return &EvaluationResult{Exceptions: matchedExceptions}, nil
		}
	}
	dataNew[compiler.ExceptionsKey] = libs.Exception{
		AllowedImages: allowedImages,
		AllowedValues: allowedValues,
	}
	match, err := p.match(ctx, dataNew, p.matchConditions)
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, nil
	}
	vars := lazy.NewMapValue(compiler.VariablesType)
	dataNew[compiler.VariablesKey] = vars
	for name, variable := range p.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, dataNew)
			if out != nil {
				return out
			}
			if err != nil {
				return types.WrapErr(err)
			}
			return nil
		})
	}
	for index, validation := range p.validations {
		out, _, err := validation.Program.ContextEval(ctx, dataNew)
		if err != nil {
			return nil, err
		}
		// evaluate only when rule fails
		if outcome, err := utils.ConvertToNative[bool](out); err == nil && !outcome {
			message := validation.Message
			if validation.MessageExpression != nil {
				if out, _, err := validation.MessageExpression.ContextEval(ctx, dataNew); err != nil {
					message = fmt.Sprintf("failed to evaluate message expression: %s", err)
				} else if msg, err := utils.ConvertToNative[string](out); err != nil {
					message = fmt.Sprintf("failed to convert message expression to string: %s", err)
				} else {
					message = msg
				}
			}
			// Add default message if empty
			if message == "" {
				message = fmt.Sprintf("CEL expression validation failed at index %d", index)
			}
			auditAnnotations := make(map[string]string, 0)
			for key, annotation := range p.auditAnnotations {
				out, _, err := annotation.ContextEval(ctx, dataNew)
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

func (p *Policy) match(
	ctx context.Context,
	data map[string]any,
	matchConditions []cel.Program,
) (bool, error) {
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
