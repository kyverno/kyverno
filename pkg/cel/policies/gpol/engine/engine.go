package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

type Engine struct {
	nsResolver engine.NamespaceResolver
	matcher    matching.Matcher
}

func NewEngine(nsResolver engine.NamespaceResolver, matcher matching.Matcher) *Engine {
	return &Engine{
		nsResolver: nsResolver,
		matcher:    matcher,
	}
}

// Handle evaluates a generating policy against the trigger in the provided request.
func (e *Engine) Handle(request engine.EngineRequest, policy Policy) (EngineResponse, error) {
	var response EngineResponse
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, err
	}
	response.Trigger = &object
	if response.Trigger.Object == nil {
		response.Trigger = &oldObject
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
		object.GetNamespace(),
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
	response.Policies = append(response.Policies, e.generate(context.TODO(), policy, attr, &request.Request, namespace, request.Context, string(object.GetUID())))
	return response, nil
}

func (e *Engine) generate(ctx context.Context, policy Policy, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context libs.Context, triggerUID string) GeneratingPolicyResponse {
	response := GeneratingPolicyResponse{
		Policy: policy.Policy,
	}
	if e.matcher != nil {
		matches, err := e.matchPolicy(policy.Policy.Spec.MatchConstraints, attr, namespace)
		if err != nil {
			response.Result = engineapi.RuleError(policy.Policy.Name, engineapi.Generation, "failed to execute matching", err, nil)
			return response
		} else if !matches {
			return response
		}
	}
	context.SetPolicyName(policy.Policy.Name)
	context.SetTriggerMetadata(request.Name, attr.GetNamespace(), triggerUID, request.Kind.Version, request.Kind.Group, request.Kind.Kind)
	generatedResources, err := policy.CompiledPolicy.Evaluate(ctx, attr, request, namespace, context)
	if err != nil {
		response.Result = engineapi.RuleError(policy.Policy.Name, engineapi.Generation, "failed to evaluate policy", err, nil)
		return response
	}
	response.Result = engineapi.RulePass(policy.Policy.Name, engineapi.Generation, "policy evaluated successfully", nil).WithGeneratedResources(generatedResources)
	return response
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
