package engine

import (
	"context"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/client-go/kubernetes"
)

type Engine interface {
	Handle(context.Context, engine.EngineRequest, Predicate) (EngineResponse, error)
}

type EngineResponse struct {
	Resource *unstructured.Unstructured
	Policies []MutatingPolicyResponse
}

type MutatingPolicyResponse struct {
	Policy *policiesv1alpha1.MutatingPolicy
	Rules  []engineapi.RuleResponse
}

type Predicate = func(policiesv1alpha1.MutatingPolicy) bool
type engineImpl struct {
	provider   Provider
	client     kubernetes.Interface
	nsResolver engine.NamespaceResolver
	matcher    matching.Matcher
}

func NewEngine(provider Provider, nsResolver engine.NamespaceResolver, client kubernetes.Interface, matcher matching.Matcher) *engineImpl {
	return &engineImpl{
		provider:   provider,
		nsResolver: nsResolver,
		client:     client,
		matcher:    matcher,
	}
}

func (e *engineImpl) Handle(ctx context.Context, request engine.EngineRequest, predicate Predicate) (EngineResponse, error) {
	var response EngineResponse
	mpols, err := e.provider.Fetch(ctx)
	if err != nil {
		return response, err
	}

	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, err
	}
	response.Resource = &object
	dryRun := false
	if request.Request.DryRun != nil {
		dryRun = *request.Request.DryRun
	}

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
		// TODO
		nil,
	)

	var namespace *corev1.Namespace
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}

	typeConverter := patch.NewTypeConverterManager(nil, e.client.Discovery().OpenAPIV3())
	for _, mpol := range mpols {
		if predicate != nil && !predicate(mpol.Policy) {
			continue
		}
		ruleResponse := e.handlePolicy(ctx, mpol, attr, namespace, typeConverter)
		response.Policies = append(response.Policies, ruleResponse)
	}
	return response, nil
}

func (e *engineImpl) handlePolicy(ctx context.Context, mpol Policy, attr admission.Attributes, namespace *corev1.Namespace, typeConverter patch.TypeConverterManager) MutatingPolicyResponse {
	ruleResponse := MutatingPolicyResponse{
		Policy: &mpol.Policy,
	}

	if e.matcher != nil {
		constraints := mpol.Policy.GetMatchConstraints()
		matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: &constraints}, attr, namespace)
		if err != nil {
			ruleResponse.Rules = handlers.WithResponses(engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil))
			return ruleResponse
		} else if !matches {
			return ruleResponse
		}
	}
	result := mpol.CompiledPolicy.Evaluate(ctx, attr, namespace, typeConverter)
	if result == nil {
		ruleResponse.Rules = append(ruleResponse.Rules, *engineapi.RuleSkip("", engineapi.Mutation, "skip", nil))
	} else if result.Error != nil {
		ruleResponse.Rules = handlers.WithResponses(engineapi.RuleError("evaluation", engineapi.Mutation, "failed to evaluate policy", result.Error, nil))
	} else {
		ruleResponse.Rules = append(ruleResponse.Rules, *engineapi.RulePass("", engineapi.Mutation, "success", nil))
	}
	return ruleResponse
}
