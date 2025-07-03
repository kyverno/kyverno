package compiler

import (
	"context"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	admission "k8s.io/apiserver/pkg/admission"
	cel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	evaluator mutating.PolicyEvaluator
	// TODO(shuting)
	exceptions []compiler.Exception
}

// compositionContext implements the CompositionContext interface
type compositionContext struct {
	context.Context
	evaluator       *mutating.PolicyEvaluator
	contextProvider libs.Context
	accumulatedCost int64
}

func (c *compositionContext) Variables(activation any) ref.Val {
	// Create lazy map using the composition environment's map type
	lazyMap := lazy.NewMapValue(c.evaluator.CompositionEnv.MapType)

	// Extract object and oldObject from the activation context
	var objectVal, oldObjectVal interface{}
	if evalActivation, ok := activation.(interface {
		ResolveName(string) (interface{}, bool)
	}); ok {
		if obj, found := evalActivation.ResolveName("object"); found {
			objectVal = obj
		}
		if oldObj, found := evalActivation.ResolveName("oldObject"); found {
			oldObjectVal = oldObj
		}
	}

	// Set up context data for variable evaluation
	ctxData := map[string]interface{}{
		compiler.GlobalContextKey: globalcontext.Context{ContextInterface: c.contextProvider},
		compiler.HttpKey:          http.Context{ContextInterface: http.NewHTTP(nil)},
		compiler.ImageDataKey:     imagedata.Context{ContextInterface: c.contextProvider},
		compiler.ResourceKey:      resource.Context{ContextInterface: c.contextProvider},
		compiler.VariablesKey:     lazyMap,
		compiler.ObjectKey:        objectVal,
		compiler.OldObjectKey:     oldObjectVal,
	}

	for name, result := range c.evaluator.CompositionEnv.CompiledVariables {
		lazyMap.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := result.Program.ContextEval(c.Context, ctxData)
			if out != nil {
				return out
			}
			if err != nil {
				return types.WrapErr(err)
			}
			return nil
		})
	}

	return lazyMap
}

func (c *compositionContext) GetAndResetCost() int64 {
	cost := c.accumulatedCost
	c.accumulatedCost = 0
	return cost
}

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	tcm TypeConverterManager,
	contextProvider libs.Context,
) *EvaluationResult {
	versionedAttributes := &admission.VersionedAttributes{
		Attributes:      attr,
		VersionedObject: attr.GetObject(),
		VersionedKind:   attr.GetKind(),
	}

	if p.evaluator.Matcher != nil {
		matchResult := p.evaluator.Matcher.Match(ctx, versionedAttributes, namespace, nil)
		if matchResult.Error != nil {
			return &EvaluationResult{Error: matchResult.Error}
		}
		if !matchResult.Matches {
			return nil
		}
	}

	compositionCtx := &compositionContext{
		Context:         ctx,
		evaluator:       &p.evaluator,
		contextProvider: contextProvider,
	}

	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	for _, patcher := range p.evaluator.Mutators {
		patchRequest := patch.Request{
			MatchedResource:     attr.GetResource(),
			VersionedAttributes: versionedAttributes,
			ObjectInterfaces:    o,
			OptionalVariables:   cel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
			Namespace:           namespace,
			TypeConverter:       tcm.GetTypeConverter(versionedAttributes.VersionedKind),
		}

		newVersionedObject, err := patcher.Patch(compositionCtx, patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			return &EvaluationResult{Error: err}
		}

		versionedAttributes.Dirty = true
		versionedAttributes.VersionedObject = newVersionedObject
	}

	return &EvaluationResult{PatchedResource: versionedAttributes.VersionedObject.(*unstructured.Unstructured)}
}

func (p *Policy) MatchesConditions(ctx context.Context, attr admission.Attributes, namespace *corev1.Namespace) bool {
	if p.evaluator.Matcher != nil {
		versionedAttributes := &admission.VersionedAttributes{
			Attributes:      attr,
			VersionedObject: attr.GetObject(),
			VersionedKind:   attr.GetKind(),
		}
		matchResult := p.evaluator.Matcher.Match(ctx, versionedAttributes, namespace, nil)
		if matchResult.Error != nil || !matchResult.Matches {
			return false
		}
		return true
	}

	return false
}
