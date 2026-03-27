package compiler

import (
	"context"
	"time"

	cel "github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/sdk/cel/utils"
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
	patchers         []Patcher
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	exceptions       []compiler.Exception
	matchConstraints *admissionregistrationv1.MatchResources
}

func (p *Policy) MatchConstraints() *admissionregistrationv1.MatchResources {
	return p.matchConstraints
}

type compositionContext struct {
	ctx             context.Context //nolint:containedctx
	variables       *lazy.MapValue
	accumulatedCost int64
}

func (c *compositionContext) Variables(activation any) ref.Val {
	return c.variables
}

func (c *compositionContext) GetAndResetCost() int64 {
	cost := c.accumulatedCost
	c.accumulatedCost = 0
	return cost
}

func (c *compositionContext) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

func (c *compositionContext) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *compositionContext) Err() error {
	return c.ctx.Err()
}

func (c *compositionContext) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
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

func (p *Policy) MatchesConditions(ctx context.Context, attr admission.Attributes, namespace *corev1.Namespace, contextProvider libs.Context) bool {
	data, err := prepareData(attr, nil, namespace)
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

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	request admissionv1.AdmissionRequest,
	tcm TypeConverterManager,
	contextProvider libs.Context,
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

	// variables also get added to the input data map
	vars := p.appendVariables(ctx, data)

	match, err := p.match(ctx, data, p.matchConditions)
	if err != nil {
		return &EvaluationResult{Error: err}
	}
	if !match {
		return nil
	}

	compositionCtx := &compositionContext{
		ctx:       ctx,
		variables: vars,
	}

	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	for _, patcher := range p.patchers {
		// do we need to create to create this punk ass type ?
		// can we just use the admission request ?
		patchRequest := patch.Request{
			MatchedResource:     attr.GetResource(),
			VersionedAttributes: versionedAttributes,
			ObjectInterfaces:    o,
			OptionalVariables:   plugincel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
			Namespace:           namespace,
			TypeConverter:       tcm.GetTypeConverter(versionedAttributes.VersionedKind),
		}

		newVersionedObject, err := patcher.Patch(compositionCtx, data, patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			return &EvaluationResult{Error: err}
		}

		versionedAttributes.Dirty = true
		versionedAttributes.VersionedObject = newVersionedObject
	}

	return &EvaluationResult{PatchedResource: versionedAttributes.VersionedObject.(*unstructured.Unstructured)}
}

func (p *Policy) GetCompiledVariables() map[string]cel.Program {
	return p.variables
}
