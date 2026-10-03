package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
)

type EngineResponse struct {
	// Resource is the resource the policy was evaluated against.
	Resource *unstructured.Unstructured
	// Match reports whether the policy's delete conditions evaluated to true.
	// Only meaningful when the returned error is nil.
	Match bool
	// PolicyMatched reports whether the resource was selected by the policy's
	// matchConstraints (resourceRules, objectSelector, namespaceSelector).
	// When false, Match is always false and the delete conditions were not
	// evaluated. Only meaningful when the returned error is nil.
	PolicyMatched bool
	// Trace explains how the policy reached this result. It is nil unless the policy was
	// compiled with tracing on (compiler.NewCompilerWithTrace), and unlike the fields above it
	// may also be set alongside a returned error, carrying what was traced before the failure.
	Trace *trace.Decision
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
	tracing := policy.CompiledPolicy != nil && policy.CompiledPolicy.Tracing()
	var scope trace.ScopeTrace
	// withTrace attaches d, with the scope and the policy/resource header filled in, to resp. It
	// is a no-op when the policy was not compiled for tracing, so untraced responses are unchanged.
	withTrace := func(resp EngineResponse, d *trace.Decision) EngineResponse {
		if !tracing || d == nil {
			return resp
		}
		d.Scope = scope
		d.PolicyName = policy.Policy.GetName()
		d.PolicyKind = policy.Policy.GetKind()
		if d.PolicyKind == "" {
			d.PolicyKind = "DeletingPolicy"
		}
		d.ResourceKind = resource.GetKind()
		d.ResourceName = resource.GetName()
		d.ResourceNamespace = resource.GetNamespace()
		resp.Trace = d
		return resp
	}
	errored := func(err error) *trace.Decision {
		return &trace.Decision{Verdict: trace.VerdictTrace{Status: trace.VerdictError, Message: err.Error()}}
	}

	var ns runtime.Object
	if resource.GetAPIVersion() != "" && resource.GetKind() != "" {
		namespace := resource.GetNamespace()

		spec := policy.Policy.GetDeletingPolicySpec()
		if spec == nil {
			err := fmt.Errorf("deleting policy %s has no spec", policy.Policy.GetName())
			return withTrace(EngineResponse{}, errored(err)), err
		}

		mapping, err := e.mapper.RESTMapping(resource.GroupVersionKind().GroupKind(), resource.GroupVersionKind().Version)
		if err != nil {
			return withTrace(EngineResponse{}, errored(err)), err
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
			scope = trace.ScopeTrace{Reason: "failed to evaluate matchConstraints: " + err.Error()}
			return withTrace(EngineResponse{}, errored(err)), err
		} else if !matches {
			// explain the same constraints matchPolicy checked (operations widened to all), and drop
			// the placeholder CREATE operation attr carries: a deletion scan has no operation
			reason := matching.Explain(deletionConstraints(spec.MatchConstraints), attr, ns, false)
			reason = strings.Replace(reason, ", operation "+string(admission.Create), "", 1)
			scope = trace.ScopeTrace{Reason: reason}
			return withTrace(EngineResponse{Match: false, PolicyMatched: false}, &trace.Decision{
				Verdict: trace.VerdictTrace{Status: trace.VerdictSkip, Message: "the policy does not apply to this resource"},
			}), nil
		}
		// not matching.Explain's "matched" wording: attr carries a placeholder CREATE operation,
		// and a scheduled deletion scan has no admission operation at all
		scope = trace.ScopeTrace{Applied: true, Reason: "matched " + describeResource(resource) + " (deletion scan, so operations are not checked)"}
	} else {
		scope = trace.ScopeTrace{Applied: true, Reason: "evaluated against a JSON payload, so no matchConstraints apply"}
	}

	result, err := policy.CompiledPolicy.Evaluate(ctx, resource, ns, e.context)
	if err != nil {
		if result != nil && result.Trace != nil {
			// Evaluate failed partway but kept what it traced, e.g. the conditions up to the error
			return withTrace(EngineResponse{}, result.Trace), err
		}
		return withTrace(EngineResponse{}, errored(err)), err
	}

	return withTrace(EngineResponse{Match: result.Result, PolicyMatched: true}, result.Trace), nil
}

func describeResource(resource unstructured.Unstructured) string {
	if ns := resource.GetNamespace(); ns != "" {
		return fmt.Sprintf("kind %s, namespace %s", resource.GetKind(), ns)
	}
	return fmt.Sprintf("kind %s, cluster-scoped", resource.GetKind())
}

// deletionConstraints returns a copy of constraints with every resource rule widened to all
// operations: a scheduled deletion scan is not an admission request, so the rules' operations
// must not filter anything out. Returns nil for nil constraints.
func deletionConstraints(constraints *admissionregistrationv1.MatchResources) *admissionregistrationv1.MatchResources {
	if constraints == nil {
		return nil
	}
	copy := constraints.DeepCopy()
	for i, rule := range copy.ResourceRules {
		rule.Operations = []admissionregistrationv1.OperationType{
			admissionregistrationv1.OperationAll,
		}
		copy.ResourceRules[i] = rule
	}
	return copy
}

func (e *Engine) matchPolicy(constraints *admissionregistrationv1.MatchResources, attr admission.Attributes, namespace runtime.Object) (bool, error) {
	if constraints == nil {
		return false, nil
	}

	matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: deletionConstraints(constraints)}, attr, namespace)
	if err != nil {
		return false, err
	}
	return matches, nil
}
