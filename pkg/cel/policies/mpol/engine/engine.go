package engine

import (
	"context"
	"strings"
	"time"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/tools/cache"
)

type Engine interface {
	Handle(context.Context, engine.EngineRequest, Predicate) (EngineResponse, error)
	Evaluate(context.Context, admission.Attributes, admissionv1.AdmissionRequest, Predicate) (EngineResponse, error)
	MatchedMutateExistingPolicies(context.Context, engine.EngineRequest) []string
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
	provider        Provider
	nsResolver      engine.NamespaceResolver
	matcher         matching.Matcher
	typeConverter   compiler.TypeConverterManager
	contextProvider libs.Context
}

func NewEngine(provider Provider, nsResolver engine.NamespaceResolver, matcher matching.Matcher, typeConverter compiler.TypeConverterManager, contextProvider libs.Context) *engineImpl {
	return &engineImpl{
		provider:        provider,
		nsResolver:      nsResolver,
		matcher:         matcher,
		typeConverter:   typeConverter,
		contextProvider: contextProvider,
	}
}

func (e *engineImpl) Evaluate(ctx context.Context, attr admission.Attributes, request admissionv1.AdmissionRequest, predicate Predicate) (EngineResponse, error) {
	mpols, err := e.provider.Fetch(ctx, true)
	if err != nil {
		return EngineResponse{}, err
	}

	var object *unstructured.Unstructured
	if o, ok := attr.GetObject().(*unstructured.Unstructured); ok {
		object = o
	}

	response := EngineResponse{
		Resource: object,
	}

	for _, mpol := range mpols {
		if predicate != nil && predicate(mpol.Policy) {
			r, patched := e.handlePolicy(ctx, mpol, attr, request, nil)
			response.Policies = append(response.Policies, r)
			if patched != nil {
				response.PatchedResource = patched
			}
		}
	}
	return response, nil
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
		ruleResponse, patchedResource := e.handlePolicy(ctx, mpol, attr, request.Request, namespace)
		response.Policies = append(response.Policies, ruleResponse)
		if patchedResource != nil {
			response.PatchedResource = patchedResource
		}
	}
	return response, nil
}

func (e *engineImpl) handlePolicy(ctx context.Context, mpol Policy, attr admission.Attributes, request admissionv1.AdmissionRequest, namespace *corev1.Namespace) (MutatingPolicyResponse, *unstructured.Unstructured) {
	ruleResponse := MutatingPolicyResponse{
		Policy: &mpol.Policy,
		Rules:  []engineapi.RuleResponse{},
	}

	startTime := time.Now()
	if e.matcher != nil {
		constraints := mpol.Policy.GetMatchConstraints()
		matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: &constraints}, attr, namespace)
		if err != nil {
			ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
			return ruleResponse, nil
		} else if !matches {
			return ruleResponse, nil
		}
	}
	result := mpol.CompiledPolicy.Evaluate(ctx, attr, namespace, request, e.typeConverter, e.contextProvider)
	if result == nil {
		ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RuleSkip("", engineapi.Mutation, "skip", nil).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
		return ruleResponse, nil
	} else if result.Error != nil {
		ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RuleError("evaluation", engineapi.Mutation, "failed to evaluate policy", result.Error, nil).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
		return ruleResponse, nil
	} else if len(result.Exceptions) > 0 {
		exceptions := make([]engineapi.GenericException, 0, len(result.Exceptions))
		var keys []string
		for i := range result.Exceptions {
			key, err := cache.MetaNamespaceKeyFunc(result.Exceptions[i])
			if err != nil {
				ruleResponse.Rules = handlers.WithResponses(engineapi.RuleError("exception", engineapi.Mutation, "failed to compute exception key", err, nil))
				return ruleResponse, nil
			}
			keys = append(keys, key)
			exceptions = append(exceptions, engineapi.NewCELPolicyException(result.Exceptions[i]))
		}
		ruleResponse.Rules = handlers.WithResponses(engineapi.RuleSkip("exception", engineapi.Mutation, "rule is skipped due to policy exception: "+strings.Join(keys, ", "), nil).WithExceptions(exceptions))
	} else {
		ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RulePass("", engineapi.Mutation, "success", nil).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
	}
	return ruleResponse, result.PatchedResource
}

func (e *engineImpl) MatchedMutateExistingPolicies(ctx context.Context, request engine.EngineRequest) []string {
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return nil
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

	return e.provider.MatchesMutateExisting(ctx, attr, namespace)
}
