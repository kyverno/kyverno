package engine

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

type Engine interface {
	Handle(context.Context, engine.EngineRequest, Predicate) (EngineResponse, error)
	MatchedMutateExistingPolicies(context.Context, engine.EngineRequest) ([]string, kyvernov1.ResourceSpec)
}

type EngineResponse struct {
	PatchedResource *unstructured.Unstructured
	Resource        *unstructured.Unstructured
	Policies        []MutatingPolicyResponse
}

func (er EngineResponse) GetPatches() []jsonpatch.JsonPatchOperation {
	originalBytes, err := er.Resource.MarshalJSON()
	if err != nil {
		return nil
	}
	patchedBytes, err := er.PatchedResource.MarshalJSON()
	if err != nil {
		return nil
	}
	patches, err := jsonpatch.CreatePatch(originalBytes, patchedBytes)
	if err != nil {
		return nil
	}
	return patches
}

type MutatingPolicyResponse struct {
	Policy *policiesv1alpha1.MutatingPolicy
	Rules  []engineapi.RuleResponse
}

type Predicate = func(policiesv1alpha1.MutatingPolicy) bool

type engineImpl struct {
	provider      Provider
	nsResolver    engine.NamespaceResolver
	matcher       matching.Matcher
	typeConverter compiler.TypeConverterManager
}

func NewEngine(provider Provider, nsResolver engine.NamespaceResolver, matcher matching.Matcher, typeConverter compiler.TypeConverterManager) *engineImpl {
	return &engineImpl{
		provider:      provider,
		nsResolver:    nsResolver,
		matcher:       matcher,
		typeConverter: typeConverter,
	}
}

func (e *engineImpl) Handle(ctx context.Context, request engine.EngineRequest, predicate Predicate) (EngineResponse, error) {
	var response EngineResponse
	mpols, err := e.provider.Fetch(ctx, false)
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

	for _, mpol := range mpols {
		if predicate != nil && !predicate(mpol.Policy) {
			continue
		}
		ruleResponse, patchedResource := e.handlePolicy(ctx, mpol, attr, namespace)
		response.Policies = append(response.Policies, ruleResponse)
		if patchedResource != nil {
			response.PatchedResource = patchedResource
		}
	}
	return response, nil
}

func (e *engineImpl) handlePolicy(ctx context.Context, mpol Policy, attr admission.Attributes, namespace *corev1.Namespace) (MutatingPolicyResponse, *unstructured.Unstructured) {
	ruleResponse := MutatingPolicyResponse{
		Policy: &mpol.Policy,
	}

	if e.matcher != nil {
		constraints := mpol.Policy.GetMatchConstraints()
		matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: &constraints}, attr, namespace)
		if err != nil {
			ruleResponse.Rules = handlers.WithResponses(engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil))
			return ruleResponse, nil
		} else if !matches {
			return ruleResponse, nil
		}
	}
	result := mpol.CompiledPolicy.Evaluate(ctx, attr, namespace, e.typeConverter)
	if result == nil {
		ruleResponse.Rules = append(ruleResponse.Rules, *engineapi.RuleSkip("", engineapi.Mutation, "skip", nil))
		return ruleResponse, nil
	} else if result.Error != nil {
		ruleResponse.Rules = handlers.WithResponses(engineapi.RuleError("evaluation", engineapi.Mutation, "failed to evaluate policy", result.Error, nil))
		return ruleResponse, nil
	} else {
		ruleResponse.Rules = append(ruleResponse.Rules, *engineapi.RulePass("", engineapi.Mutation, "success", nil))
	}
	return ruleResponse, result.PatchedResource
}

func (e *engineImpl) MatchedMutateExistingPolicies(ctx context.Context, request engine.EngineRequest) ([]string, kyvernov1.ResourceSpec) {
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return nil, kyvernov1.ResourceSpec{}
	}
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

	return e.provider.MatchesMutateExisting(ctx, attr, namespace),
		kyvernov1.ResourceSpec{
			APIVersion: object.GetAPIVersion(),
			Kind:       object.GetKind(),
			Namespace:  object.GetNamespace(),
			UID:        object.GetUID(),
		}
}
