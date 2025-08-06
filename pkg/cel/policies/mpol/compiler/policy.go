package compiler

import (
	"context"
	"fmt"
	"reflect"
	"time"

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
	evaluator  mutating.PolicyEvaluator
	exceptions []compiler.Exception
}

type compositionContext struct {
	ctx             context.Context //nolint:containedctx
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
			out, _, err := result.Program.ContextEval(c.ctx, ctxData)
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

	if len(p.exceptions) > 0 {
		matchedExceptions, err := p.matchExceptions(ctx, attr, request, namespace)
		if err != nil {
			return &EvaluationResult{Error: err}
		}
		if len(matchedExceptions) > 0 {
			return &EvaluationResult{Exceptions: matchedExceptions}
		}
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
		ctx:             ctx,
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

func (p *Policy) matchExceptions(ctx context.Context, attr admission.Attributes, request admissionv1.AdmissionRequest, namespace *corev1.Namespace) ([]*policiesv1alpha1.PolicyException, error) {
	var errs []error
	matchedExceptions := make([]*policiesv1alpha1.PolicyException, 0)
	objectVal, err := utils.ObjectToResolveVal(attr.GetObject())
	if err != nil {
		return nil, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
	}
	namespaceVal, err := utils.ObjectToResolveVal(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
	}

	data := map[string]any{
		compiler.NamespaceObjectKey: namespaceVal,
		compiler.ObjectKey:          objectVal,
	}

	if attr.GetOldObject() != nil {
		oldObjectVal, err := utils.ObjectToResolveVal(attr.GetOldObject())
		if err != nil {
			return nil, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
		}
		data[compiler.OldObjectKey] = oldObjectVal
	}

	if reflect.DeepEqual(request, admissionv1.AdmissionRequest{}) {
		requestVal, err := utils.ConvertObjectToUnstructured(request)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
		}
		data[compiler.RequestKey] = requestVal
	}
	for _, polex := range p.exceptions {
		for _, condition := range polex.MatchConditions {
			out, _, err := condition.ContextEval(ctx, data)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			result, err := utils.ConvertToNative[bool](out)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if !result {
				break
			}
			if err := multierr.Combine(errs...); err == nil {
				matchedExceptions = append(matchedExceptions, polex.Exception)
			}
		}
	}
	return matchedExceptions, multierr.Combine(errs...)
}
