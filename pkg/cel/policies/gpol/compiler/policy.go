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
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/template"
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
	namespace        string
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	generations      []Generation
	auditAnnotations map[string]cel.Program
	exceptions       []compiler.Exception
	matchConstraints *admissionregistrationv1.MatchResources
}

// Generation is a compiled generate entry: either a CEL expression program or
// a YAML template. Exactly one of the two is set.
type Generation struct {
	expression cel.Program
	template   *template.Template
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
		if generation.template != nil {
			if err := p.applyTemplate(ctx, generation.template, dataNew, data.Context); err != nil {
				return nil, err
			}
			continue
		}
		_, _, err := generation.expression.ContextEval(ctx, dataNew)
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

// applyTemplate renders a generation template and feeds the resulting
// resources into the same generation runtime path used by generator.Apply.
// The target namespace of each resource is taken from its rendered
// metadata.namespace. For a namespaced policy, resources without a namespace
// default to the policy namespace and cross-namespace generation is denied,
// mirroring the namespaced generator.Apply semantics.
func (p *Policy) applyTemplate(ctx context.Context, tpl *template.Template, activation map[string]any, libsCtx libs.Context) error {
	resources, err := tpl.Render(ctx, activation)
	if err != nil {
		return fmt.Errorf("failed to render generation template: %w", err)
	}
	namespaces := make([]string, 0, 1)
	grouped := map[string][]map[string]any{}
	for _, resource := range resources {
		obj := unstructured.Unstructured{Object: resource}
		namespace := obj.GetNamespace()
		if p.namespace != "" {
			if namespace == "" {
				namespace = p.namespace
			} else if namespace != p.namespace {
				return fmt.Errorf("cross-namespace generation denied: a policy in namespace %q cannot generate resources into namespace %q", p.namespace, namespace)
			}
		}
		if _, ok := grouped[namespace]; !ok {
			namespaces = append(namespaces, namespace)
		}
		grouped[namespace] = append(grouped[namespace], resource)
	}
	for _, namespace := range namespaces {
		if err := libsCtx.GenerateResources(namespace, grouped[namespace]); err != nil {
			return fmt.Errorf("failed to generate resources: %w", err)
		}
	}
	return nil
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
