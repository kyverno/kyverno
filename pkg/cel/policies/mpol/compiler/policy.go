package compiler

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/compiler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	admission "k8s.io/apiserver/pkg/admission"
	cel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

type Policy struct {
	evaluator  mutating.PolicyEvaluator
	exceptions []compiler.Exception
}

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	gvr schema.GroupVersionResource,
	namespace *corev1.Namespace,
	tcm patch.TypeConverterManager,
) (*EvaluationResult, error) {
	if p.evaluator.CompositionEnv != nil {
		ctx = p.evaluator.CompositionEnv.CreateContext(ctx)
	}
	versionedAttributes, _ := admission.NewVersionedAttributes(attr, attr.GetKind(), nil)
	matchResult := p.evaluator.Matcher.Match(ctx, versionedAttributes, namespace, nil)
	if matchResult.Error != nil {
		return nil, matchResult.Error
	}
	if !matchResult.Matches {
		return nil, nil
	}

	o := admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	for _, patcher := range p.evaluator.Mutators {
		patchRequest := patch.Request{
			MatchedResource:     gvr,
			VersionedAttributes: versionedAttributes,
			ObjectInterfaces:    o,
			OptionalVariables:   cel.OptionalVariableBindings{VersionedParams: nil, Authorizer: nil},
			Namespace:           namespace,
			TypeConverter:       tcm.GetTypeConverter(versionedAttributes.VersionedKind),
		}
		newVersionedObject, err := patcher.Patch(ctx, patchRequest, celconfig.RuntimeCELCostBudget)
		if err != nil {
			return nil, err
		}

		switch versionedAttributes.VersionedObject.(type) {
		case *unstructured.Unstructured:
			// No conversion needed before defaulting for the patch object if the admitted object is unstructured.
		default:
			// Before defaulting, if the admitted object is a typed object, convert unstructured patch result back to a typed object.
			newVersionedObject, err = o.GetObjectConvertor().ConvertToVersion(newVersionedObject, versionedAttributes.GetKind().GroupVersion())
			if err != nil {
				return nil, err
			}
		}
		o.GetObjectDefaulter().Default(newVersionedObject)
	}

	return nil, nil
}
