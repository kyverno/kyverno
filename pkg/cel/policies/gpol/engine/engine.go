package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

type Engine struct {
	provider   Provider
	nsResolver engine.NamespaceResolver
	matcher    matching.Matcher
}

func NewEngine(provider Provider, nsResolver engine.NamespaceResolver, matcher matching.Matcher) *Engine {
	return &Engine{
		provider:   provider,
		nsResolver: nsResolver,
		matcher:    matcher,
	}
}

// Generate evaluates a generating policy against the trigger in the provided request.
func (e *Engine) Generate(request engine.EngineRequest, policyName string) error {
	// fetch the compiled policy
	policy, err := e.provider.Get(context.TODO(), policyName)
	if err != nil {
		return err
	}
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return err
	}
	// default dry run
	dryRun := false
	if request.Request.DryRun != nil {
		dryRun = *request.Request.DryRun
	}
	// create admission attributes
	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(request.Request.Kind),
		request.Request.Namespace,
		request.Request.Name,
		schema.GroupVersionResource(request.Request.Resource),
		request.Request.SubResource,
		admission.Operation(request.Request.Operation),
		nil,
		dryRun,
		nil,
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}
	// check if the policy matches the trigger
	if e.matcher != nil {
		matches, err := e.matchPolicy(policy.Policy.Spec.MatchConstraints, attr, namespace)
		if err != nil {
			return err
		}
		if !matches {
			return nil
		}
	}
	// evaluate the policy
	err = policy.CompiledPolicy.Evaluate(
		context.TODO(),
		attr,
		&request.Request,
		namespace,
		request.Context,
	)
	if err != nil {
		return err
	}
	return nil
}

func (e *Engine) matchPolicy(constraints *admissionregistrationv1.MatchResources, attr admission.Attributes, namespace runtime.Object) (bool, error) {
	if constraints == nil {
		return false, nil
	}
	matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: constraints}, attr, namespace)
	if err != nil {
		return false, err
	}
	return matches, nil
}
