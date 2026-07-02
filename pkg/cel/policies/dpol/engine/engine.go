package engine

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
)

type EngineResponse struct {
	Resource      *unstructured.Unstructured
	Match         bool
	PolicyMatched bool
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
	var ns runtime.Object
	if resource.GetAPIVersion() != "" && resource.GetKind() != "" {
		namespace := resource.GetNamespace()

		spec := policy.Policy.GetDeletingPolicySpec()
		if spec == nil {
			return EngineResponse{}, fmt.Errorf("deleting policy %s has no spec", policy.Policy.GetName())
		}

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
			admission.Create,
			nil,
			false,
			nil,
		)

		if namespace != "" {
			ns = e.nsResolver(namespace)
		} else if resource.GroupVersionKind().Group == "" && resource.GetKind() == "Namespace" {
			// For Namespace resources (cluster-scoped), build ns from the resource itself so
			// that namespaceSelector and namespaceObject work correctly even when the resolver
			// cannot return the namespace (CLI, cache-miss, or test paths).
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   resource.GetName(),
					Labels: resource.GetLabels(),
				},
			}
			// Prefer the resolver's copy if available (may carry additional metadata).
			if resolved := e.nsResolver(resource.GetName()); resolved != nil {
				ns = resolved
			}
		}

		if matches, err := e.matchPolicy(spec.MatchConstraints, attr, ns); err != nil {
			return EngineResponse{}, err
		} else if !matches {
			return EngineResponse{Match: false, PolicyMatched: false}, nil
		}
	}

	result, err := policy.CompiledPolicy.Evaluate(ctx, resource, ns, e.context)
	if err != nil {
		return EngineResponse{}, err
	}

	return EngineResponse{Match: result.Result, PolicyMatched: true}, nil
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
