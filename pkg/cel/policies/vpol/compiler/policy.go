package compiler

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	mode             policiesv1alpha1.EvaluationMode
	failurePolicy    admissionregistrationv1.FailurePolicyType
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	validations      []compiler.Validation
	auditAnnotations map[string]cel.Program
	exceptions       []compiler.Exception
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
	case policiesv1alpha1.EvaluationModeJSON:
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
	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1alpha1.PolicyException, 0)
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, data.Namespace, data.Object, data.OldObject, data.Request, polex.MatchConditions)
			if err != nil {
				return nil, err
			}
			if match {
				matchedExceptions = append(matchedExceptions, polex.Exception)
			}
		}
		if len(matchedExceptions) > 0 {
			return &EvaluationResult{Exceptions: matchedExceptions}, nil
		}
	}
	match, err := p.match(ctx, data.Namespace, data.Object, data.OldObject, data.Request, p.matchConditions)
	if err != nil {
		return nil, err
	}
	if !match {
		return nil, nil
	}
	vars := lazy.NewMapValue(compiler.VariablesType)
	dataNew := map[string]any{
		compiler.GlobalContextKey:   globalcontext.Context{ContextInterface: data.Context},
		compiler.HttpKey:            http.Context{ContextInterface: http.NewHTTP(nil)},
		compiler.ImageDataKey:       imagedata.Context{ContextInterface: data.Context},
		compiler.NamespaceObjectKey: data.Namespace,
		compiler.ObjectKey:          data.Object,
		compiler.OldObjectKey:       data.OldObject,
		compiler.RequestKey:         data.Request,
		compiler.ResourceKey:        resource.Context{ContextInterface: data.Context},
		compiler.VariablesKey:       vars,
	}
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
	namespaceVal any,
	objectVal any,
	oldObjectVal any,
	requestVal any,
	matchConditions []cel.Program,
) (bool, error) {
	data := map[string]any{
		compiler.NamespaceObjectKey: namespaceVal,
		compiler.ObjectKey:          objectVal,
		compiler.OldObjectKey:       oldObjectVal,
		compiler.RequestKey:         requestVal,
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
