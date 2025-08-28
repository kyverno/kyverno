package engine

import (
	"context"

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
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

	response := EngineResponse{}

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

	var object, oldObject unstructured.Unstructured
	var admissionRequest admissionv1.AdmissionRequest
	var dryRun bool
	var namespaceName string

	// Handle JSON payload vs Kubernetes admission request
	if request.JsonPayload != nil {
		// Handle non-K8s JSON payload
		object = *request.JsonPayload
		oldObject = unstructured.Unstructured{} // Empty for JSON payloads

		// Create minimal admission request for JSON payload
		admissionRequest = admissionv1.AdmissionRequest{
			Kind:      metav1.GroupVersionKind{Kind: "JSONPayload"},
			Resource:  metav1.GroupVersionResource{Resource: "jsonpayloads"},
			Name:      "json-payload",
			Namespace: "",
			Operation: admissionv1.Create,
			DryRun:    &dryRun,
		}
		namespaceName = ""
	} else {
		// Handle Kubernetes admission request
		object, oldObject, err = admissionutils.ExtractResources(nil, request.Request)
		if err != nil {
			return response, err
		}
		admissionRequest = request.Request
		if request.Request.DryRun != nil {
			dryRun = *request.Request.DryRun
		}
		namespaceName = request.Request.Namespace
	}

	response.Resource = &object

	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(admissionRequest.Kind),
		admissionRequest.Namespace,
		admissionRequest.Name,
		schema.GroupVersionResource(admissionRequest.Resource),
		admissionRequest.SubResource,
		admission.Operation(admissionRequest.Operation),
		nil,
		dryRun,
		// TODO
		nil,
	)

	var namespace *corev1.Namespace
	if namespaceName != "" {
		namespace = e.nsResolver(namespaceName)
	}

	for _, mpol := range mpols {
		if predicate != nil && !predicate(mpol.Policy) {
			continue
		}
		ruleResponse, patchedResource := e.handlePolicy(ctx, mpol, attr, admissionRequest, namespace)
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
	result := mpol.CompiledPolicy.Evaluate(ctx, attr, namespace, request, e.typeConverter, e.contextProvider)
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

func (e *engineImpl) MatchedMutateExistingPolicies(ctx context.Context, request engine.EngineRequest) []string {
	var object, oldObject unstructured.Unstructured
	var admissionRequest admissionv1.AdmissionRequest
	var dryRun bool
	var namespaceName string

	// Handle JSON payload vs Kubernetes admission request
	if request.JsonPayload != nil {
		// Handle non-K8s JSON payload
		object = *request.JsonPayload
		oldObject = unstructured.Unstructured{} // Empty for JSON payloads

		// Create minimal admission request for JSON payload
		admissionRequest = admissionv1.AdmissionRequest{
			Kind:      metav1.GroupVersionKind{Kind: "JSONPayload"},
			Resource:  metav1.GroupVersionResource{Resource: "jsonpayloads"},
			Name:      "json-payload",
			Namespace: "",
			Operation: admissionv1.Create,
			DryRun:    &dryRun,
		}
		namespaceName = ""
	} else {
		// Handle Kubernetes admission request
		var err error
		object, oldObject, err = admissionutils.ExtractResources(nil, request.Request)
		if err != nil {
			return nil
		}
		admissionRequest = request.Request
		if request.Request.DryRun != nil {
			dryRun = *request.Request.DryRun
		}
		namespaceName = request.Request.Namespace
	}

	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(admissionRequest.Kind),
		admissionRequest.Namespace,
		admissionRequest.Name,
		schema.GroupVersionResource(admissionRequest.Resource),
		admissionRequest.SubResource,
		admission.Operation(admissionRequest.Operation),
		nil,
		dryRun,
		// TODO
		nil,
	)

	var namespace *corev1.Namespace
	if namespaceName != "" {
		namespace = e.nsResolver(namespaceName)
	}

	return e.provider.MatchesMutateExisting(ctx, attr, namespace)
}
