package compiler

import (
	"context"
	"fmt"

	cel "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	admission "k8s.io/apiserver/pkg/admission"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	patchers              []Patcher
	matchConditions       []cel.Program
	targetMatchConditions []cel.Program
	targetExpression      cel.Program
	variables             map[string]cel.Program
	auditAnnotations      map[string]cel.Program
	exceptions            []compiler.Exception
	matchConstraints      *admissionregistrationv1.MatchResources
}

func (p *Policy) MatchConstraints() *admissionregistrationv1.MatchResources {
	return p.matchConstraints
}

func (p *Policy) match(ctx context.Context, data map[string]any, matchConditions []cel.Program) (bool, error) {
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
	if err := multierr.Combine(errs...); err != nil {
		return false, err
	}

	return true, nil
}

func (p *Policy) appendVariables(ctx context.Context, data map[string]any) *lazy.MapValue {
	vars := lazy.NewMapValue(compiler.VariablesType)
	data[compiler.VariablesKey] = vars

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

	return vars
}

func (p *Policy) MatchesConditions(ctx context.Context, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace *corev1.Namespace, contextProvider libs.Context) bool {
	data, err := prepareData(attr, request, namespace)
	if err != nil {
		return false
	}

	p.appendVariables(ctx, data)

	result, err := p.match(ctx, data, p.matchConditions)
	if err != nil {
		return false
	}

	return result
}

func (p *Policy) EvaluateTargetExpression(ctx context.Context, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace *corev1.Namespace) (map[string]interface{}, error) {
	if p.targetExpression == nil {
		return nil, nil
	}
	data, err := prepareData(attr, request, namespace)
	if err != nil {
		return nil, err
	}
	p.appendVariables(ctx, data)
	out, _, err := p.targetExpression.ContextEval(ctx, data)
	if err != nil {
		return nil, err
	}
	return utils.ConvertToNative[map[string]interface{}](out)
}

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	request admissionv1.AdmissionRequest,
	tcm TypeConverterManager,
	contextProvider libs.Context,
) *EvaluationResult {
	return p.evaluate(ctx, attr, namespace, request, tcm, false)
}

func (p *Policy) EvaluateTarget(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	request admissionv1.AdmissionRequest,
	tcm TypeConverterManager,
	contextProvider libs.Context,
) *EvaluationResult {
	return p.evaluate(ctx, attr, namespace, request, tcm, true)
}

func (p *Policy) evaluate(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	request admissionv1.AdmissionRequest,
	tcm TypeConverterManager,
	target bool,
) *EvaluationResult {
	versionedAttributes := &admission.VersionedAttributes{
		Attributes:      attr,
		VersionedObject: attr.GetObject(),
		VersionedKind:   attr.GetKind(),
	}
	data, err := prepareData(attr, &request, namespace)
	if err != nil {
		return &EvaluationResult{Error: err}
	}

	allowedImages := make([]string, 0)
	allowedValues := make([]string, 0)
	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1beta1.PolicyException, 0)
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, data, polex.MatchConditions)
			if err != nil {
				return &EvaluationResult{Error: err}
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
			return &EvaluationResult{Exceptions: matchedExceptions}
		}
	}
	data[compiler.ExceptionsKey] = libs.Exception{
		AllowedImages: allowedImages,
		AllowedValues: allowedValues,
	}

	if target {
		p.appendVariables(ctx, data)
		match, err := p.match(ctx, data, p.targetMatchConditions)
		if err != nil {
			return &EvaluationResult{Error: err}
		}
		if !match {
			return nil
		}
	} else {
		// variables are lazily bound and remain visible to trigger match conditions
		// for backward compatibility with existing policies
		p.appendVariables(ctx, data)
		match, err := p.match(ctx, data, p.matchConditions)
		if err != nil {
			return &EvaluationResult{Error: err}
		}
		if !match {
			return nil
		}
	}

	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	for _, patcher := range p.patchers {
		patchRequest := patch.Request{
			MatchedResource:     attr.GetResource(),
			VersionedAttributes: versionedAttributes,
			ObjectInterfaces:    o,
			OptionalVariables:   plugincel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
			Namespace:           namespace,
			TypeConverter:       tcm.GetTypeConverter(versionedAttributes.VersionedKind),
		}

		newVersionedObject, err := patcher.Patch(ctx, data, patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			return &EvaluationResult{Error: err}
		}

		versionedAttributes.Dirty = true
		versionedAttributes.VersionedObject = newVersionedObject

		// the program data (the object) is supplied through the data map. we need to update
		// the map to get the patched object from the previous patch
		data[compiler.ObjectKey] = newVersionedObject.(*unstructured.Unstructured).Object
	}

	auditAnnotations, err := p.evaluateAuditAnnotations(ctx, data)
	if err != nil {
		return &EvaluationResult{Error: err}
	}

	return &EvaluationResult{
		PatchedResource:  versionedAttributes.VersionedObject.(*unstructured.Unstructured),
		AuditAnnotations: auditAnnotations,
	}
}

// evaluateAuditAnnotations evaluates each auditAnnotation valueExpression and returns the
// resulting key/value pairs to be surfaced as report result properties. Empty results are omitted.
func (p *Policy) evaluateAuditAnnotations(ctx context.Context, data map[string]any) (map[string]string, error) {
	if len(p.auditAnnotations) == 0 {
		return nil, nil
	}
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

func (p *Policy) GetCompiledVariables() map[string]cel.Program {
	return p.variables
}
