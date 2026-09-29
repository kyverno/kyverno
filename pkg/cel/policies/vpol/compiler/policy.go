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
	"github.com/kyverno/sdk/extensions/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
	"k8s.io/client-go/tools/cache"
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
	requestMapFn func() (map[string]any, error),
	context libs.Context,
) (*EvaluationResult, error) {
	switch p.mode {
	case policieskyvernoio.EvaluationModeJSON:
		return p.evaluateJson(ctx, json)
	default:
		return p.evaluateKubernetes(ctx, attr, request, namespace, requestMapFn, context)
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
	requestMapFn func() (map[string]any, error),
	context libs.Context,
) (*EvaluationResult, error) {
	data, err := prepareK8sData(attr, request, namespace, requestMapFn, context)
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
	var refused *RefusedException
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1beta1.PolicyException, 0)
		fullExemptionFound := false
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, dataNew, polex.MatchConditions)
			if err != nil {
				if fullExemptionFound {
					// exception already granted; a broken later exception must not negate it
					continue
				}
				return nil, err
			}
			if !match {
				continue
			}
			// controls gate the bypass, not the resource: one that fails grants nothing, another
			// exception may still cover it, and if none does the policy runs as usual. Only the
			// first refusal is kept, deterministic because exceptions are compiled sorted.
			if refusal := p.evaluateExceptionValidations(ctx, dataNew, polex); refusal != nil {
				if refused == nil {
					refused = refusal
				}
				continue
			}
			matchedExceptions = append(matchedExceptions, polex.Exception)
			if len(polex.Exception.Spec.Images) == 0 && len(polex.Exception.Spec.AllowedValues) == 0 {
				fullExemptionFound = true
			} else if !fullExemptionFound {
				// partial scopes are irrelevant once a full exemption is granted
				allowedImages = append(allowedImages, polex.Exception.Spec.Images...)
				allowedValues = append(allowedValues, polex.Exception.Spec.AllowedValues...)
			}
		}
		if fullExemptionFound {
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
			return &EvaluationResult{Error: err, Index: index}, nil
		}
		if outcome, err := utils.ConvertToNative[bool](out); err == nil && !outcome {
			message := p.resolveMessage(ctx, dataNew, validation, fmt.Sprintf("CEL expression validation failed at index %d", index))
			auditAnnotations, err := p.evaluateAuditAnnotations(ctx, dataNew)
			if err != nil {
				return &EvaluationResult{Error: err, Index: index}, nil
			}
			return &EvaluationResult{
				Result:           outcome,
				Message:          message,
				Index:            index,
				AuditAnnotations: auditAnnotations,
				RefusedException: refused,
			}, nil
		} else if err != nil {
			return &EvaluationResult{Error: err, Index: index}, nil
		}
	}
	auditAnnotations, err := p.evaluateAuditAnnotations(ctx, dataNew)
	if err != nil {
		return nil, err
	}
	return &EvaluationResult{Result: true, AuditAnnotations: auditAnnotations}, nil
}

// resolveMessage returns the message to report for a failed validation, preferring
// messageExpression over the static message and falling back when neither yields anything.
func (p *Policy) resolveMessage(
	ctx context.Context,
	data map[string]any,
	validation compiler.Validation,
	fallback string,
) string {
	message := validation.Message
	if validation.MessageExpression != nil {
		out, _, err := validation.MessageExpression.ContextEval(ctx, data)
		if err != nil {
			return fmt.Sprintf("failed to evaluate message expression: %s", err)
		}
		msg, err := utils.ConvertToNative[string](out)
		if err != nil {
			return fmt.Sprintf("failed to convert message expression to string: %s", err)
		}
		message = msg
	}
	if message == "" {
		return fallback
	}
	return message
}

// evaluateExceptionValidations evaluates the compensating controls of an exception already known
// to match. nil means every control passed and the bypass is granted; otherwise the refusal
// carries the failing control's message, or its error when a control could not be evaluated.
func (p *Policy) evaluateExceptionValidations(
	ctx context.Context,
	data map[string]any,
	polex compiler.Exception,
) *RefusedException {
	for index, validation := range polex.Validations {
		out, _, err := validation.Program.ContextEval(ctx, data)
		if err != nil {
			return &RefusedException{Exception: polex.Exception, Error: err}
		}
		outcome, err := utils.ConvertToNative[bool](out)
		if err != nil {
			return &RefusedException{Exception: polex.Exception, Error: err}
		}
		if !outcome {
			fallback := fmt.Sprintf(
				"compensating control at index %d failed for policy exception %s",
				index, cache.MetaObjectToName(polex.Exception),
			)
			return &RefusedException{
				Exception: polex.Exception,
				Message:   p.resolveMessage(ctx, data, validation, fallback),
			}
		}
	}
	return nil
}

func (p *Policy) evaluateAuditAnnotations(ctx context.Context, data map[string]any) (map[string]string, error) {
	auditAnnotations := make(map[string]string, len(p.auditAnnotations))
	for key, annotation := range p.auditAnnotations {
		out, _, err := annotation.ContextEval(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate auditAnnotation '%s': %w", key, err)
		}
		if outcome, err := utils.ConvertToNative[string](out); err == nil && outcome != "" {
			auditAnnotations[key] = outcome
		} else if err != nil {
			return nil, fmt.Errorf("failed to convert auditAnnotation '%s' expression: %w", key, err)
		}
	}
	return auditAnnotations, nil
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
