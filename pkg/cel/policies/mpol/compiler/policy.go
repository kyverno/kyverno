package compiler

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/compiler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	admission "k8s.io/apiserver/pkg/admission"
	cel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

type Policy struct {
	evaluator mutating.PolicyEvaluator
	// TODO(shuting)
	exceptions []compiler.Exception
}

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	namespace *corev1.Namespace,
	tcm patch.TypeConverterManager,
) *EvaluationResult {
	if p.evaluator.CompositionEnv != nil {
		ctx = p.evaluator.CompositionEnv.CreateContext(ctx)
	}

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
		newVersionedObject, err := patcher.Patch(ctx, patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			return &EvaluationResult{Error: err}
		}

		// Always treat the object as unstructured
		versionedAttributes.Dirty = true
		versionedAttributes.VersionedObject = newVersionedObject
	}

	return &EvaluationResult{PatchedResource: versionedAttributes.VersionedObject.(*unstructured.Unstructured)}
}
