package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
)

type EngineResponse struct {
	Resource *unstructured.Unstructured
	Match    bool
}

type Engine struct {
	nsResolver engine.NamespaceResolver
	matcher    matching.Matcher
	mapper     meta.RESTMapper
	context    libs.Context
}

func NewEngine(nsResolver engine.NamespaceResolver, mapper meta.RESTMapper, context libs.Context, matcher matching.Matcher) *Engine {
	return &Engine{
		nsResolver: nsResolver,
		matcher:    matcher,
		context:    context,
		mapper:     mapper,
	}
}

func (e *Engine) Handle(ctx context.Context, policy Policy, resource unstructured.Unstructured) (EngineResponse, error) {
	if resource.GetAPIVersion() != "" && resource.GetKind() != "" {
		namespace := resource.GetNamespace()

		mapping, err := e.mapper.RESTMapping(resource.GroupVersionKind().GroupKind(), resource.GroupVersionKind().Version)
		if err != nil {
			return EngineResponse{}, err
		}

		// create admission attributes
		attr := admission.NewAttributesRecord(
			&resource,
			nil,
			resource.GroupVersionKind(),
			namespace,
			resource.GetName(),
			mapping.Resource,
			"",
			"",
			nil,
			false,
			nil,
		)

		var ns runtime.Object
		if namespace != "" {
			ns = e.nsResolver(namespace)
		}

		if matches, err := e.matchPolicy(policy.Policy.Spec.MatchConstraints, attr, ns); err != nil {
			return EngineResponse{}, err
		} else if !matches {
			return EngineResponse{Match: false}, err
		}
	}

	result, err := policy.CompiledPolicy.Evaluate(ctx, resource, e.context)
	if err != nil {
		return EngineResponse{}, err
	}

	return EngineResponse{Match: result.Result}, nil
}

func (e *Engine) matchPolicy(constraints *admissionregistrationv1.MatchResources, attr admission.Attributes, namespace runtime.Object) (bool, error) {
	if constraints == nil {
		return false, nil
	}

	copy := constraints.DeepCopy()
	for i, rule := range copy.ResourceRules {
		rule.Operations = []admissionregistrationv1.OperationType{
			admissionregistrationv1.OperationAll,
		}

		copy.ResourceRules[i] = rule
	}

	matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: copy}, attr, namespace)
	if err != nil {
		return false, err
	}
	return matches, nil
}
