package compiler

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/sdk/extensions/cel/libs/generator"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	generations      []cel.Program
	auditAnnotations map[string]cel.Program
	exceptions       []compiler.Exception
	matchConstraints *admissionregistrationv1.MatchResources
}

func (p *Policy) MatchConstraints() *admissionregistrationv1.MatchResources {
	return p.matchConstraints
}

// EvaluationResult is returned by Evaluate and carries generated resources, matched exceptions and
// evaluated audit annotations (to be surfaced as report result properties).
type EvaluationResult struct {
	GeneratedResources []*unstructured.Unstructured
	Exceptions         []*policiesv1beta1.PolicyException
	AuditAnnotations   map[string]string
}

func (p *Policy) evaluateAuditAnnotations(ctx context.Context, data map[string]any) (map[string]string, error) {
	annotations := make(map[string]string, len(p.auditAnnotations))
	for key, prog := range p.auditAnnotations {
		out, _, err := prog.ContextEval(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate auditAnnotation %q: %w", key, err)
		}
		if outcome, err := utils.ConvertToNative[string](out); err == nil && outcome != "" {
			annotations[key] = outcome
		} else if err != nil {
			return nil, fmt.Errorf("failed to convert auditAnnotation %q expression: %w", key, err)
		}
	}
	return annotations, nil
}

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
) (*EvaluationResult, error) {
	data, err := prepareData(attr, request, namespace, context)
	if err != nil {
		return nil, err
	}
	// Ensure generated resources are always cleared, even on early returns
	// (exception-only match, match failure, errors, etc.), to prevent state leaks.
	defer data.Context.ClearGeneratedResources()

	allowedImages := make([]string, 0)
	allowedValues := make([]string, 0)
	dataNew := map[string]any{
		compiler.NamespaceObjectKey: data.Namespace,
		compiler.ObjectKey:          data.Object,
		compiler.OldObjectKey:       data.OldObject,
		compiler.RequestKey:         data.Request,
		compiler.ResourceKey:        resource.Context{ContextInterface: data.Context},
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
	dataNew[compiler.GeneratorKey] = generator.Context{ContextInterface: data.Context}
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
	for _, generation := range p.generations {
		_, _, err := generation.ContextEval(ctx, dataNew)
		if err != nil {
			return nil, err
		}
	}
	auditAnnotations, err := p.evaluateAuditAnnotations(ctx, dataNew)
	if err != nil {
		return nil, err
	}
	generatedResources := data.Context.GetGeneratedResources()
	return &EvaluationResult{
		GeneratedResources: generatedResources,
		AuditAnnotations:   auditAnnotations,
	}, nil
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
	} else {
		return false, err
	}
}
