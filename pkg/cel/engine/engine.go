package engine

import (
	"context"
	"fmt"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"
)

type EngineRequest struct {
	request admissionv1.AdmissionRequest
	context contextlib.ContextInterface
}

func RequestFromAdmission(context contextlib.ContextInterface, request admissionv1.AdmissionRequest) EngineRequest {
	return EngineRequest{
		request: request,
		context: context,
	}
}

func Request(
	context contextlib.ContextInterface,
	gvk schema.GroupVersionKind,
	gvr schema.GroupVersionResource,
	subResource string,
	name string,
	namespace string,
	operation admissionv1.Operation,
	// userInfo authenticationv1.UserInfo,
	object runtime.Object,
	oldObject runtime.Object,
	dryRun bool,
	options runtime.Object,
) EngineRequest {
	request := admissionv1.AdmissionRequest{
		Kind:               metav1.GroupVersionKind(gvk),
		Resource:           metav1.GroupVersionResource(gvr),
		SubResource:        subResource,
		RequestKind:        ptr.To(metav1.GroupVersionKind(gvk)),
		RequestResource:    ptr.To(metav1.GroupVersionResource(gvr)),
		RequestSubResource: subResource,
		Name:               name,
		Namespace:          namespace,
		Operation:          operation,
		// UserInfo: userInfo,
		Object:    runtime.RawExtension{Object: object},
		OldObject: runtime.RawExtension{Object: oldObject},
		DryRun:    &dryRun,
		Options:   runtime.RawExtension{Object: options},
	}
	return RequestFromAdmission(context, request)
}

func (r *EngineRequest) AdmissionRequest() admissionv1.AdmissionRequest {
	return r.request
}

type EngineResponse struct {
	Resource *unstructured.Unstructured
	Policies []PolicyResponse
}

type PolicyResponse struct {
	Actions sets.Set[admissionregistrationv1.ValidationAction]
	Policy  kyvernov2alpha1.ValidatingPolicy
	Rules   []engineapi.RuleResponse
}

type Engine interface {
	Handle(context.Context, EngineRequest) (EngineResponse, error)
}

type NamespaceResolver = func(string) *corev1.Namespace

type engine struct {
	provider   Provider
	nsResolver NamespaceResolver
	matcher    matching.Matcher
}

func NewEngine(provider Provider, nsResolver NamespaceResolver, matcher matching.Matcher) Engine {
	return &engine{
		provider:   provider,
		nsResolver: nsResolver,
		matcher:    matcher,
	}
}

func (e *engine) Handle(ctx context.Context, request EngineRequest) (EngineResponse, error) {
	var response EngineResponse
	// fetch compiled policies
	policies, err := e.provider.CompiledPolicies(ctx)
	if err != nil {
		return response, err
	}
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.request)
	if err != nil {
		return response, err
	}
	response.Resource = &object
	if response.Resource.Object == nil {
		response.Resource = &oldObject
	}
	// default dry run
	dryRun := false
	if request.request.DryRun != nil {
		dryRun = *request.request.DryRun
	}
	// create admission attributes
	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(request.request.Kind),
		request.request.Namespace,
		request.request.Name,
		schema.GroupVersionResource(request.request.Resource),
		request.request.SubResource,
		admission.Operation(request.request.Operation),
		nil,
		dryRun,
		// TODO
		nil,
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}
	// evaluate policies
	for _, policy := range policies {
		response.Policies = append(response.Policies, e.handlePolicy(ctx, policy, attr, &request.request, namespace, request.context))
	}
	return response, nil
}

func (e *engine) handlePolicy(ctx context.Context, policy CompiledPolicy, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context contextlib.ContextInterface) PolicyResponse {
	response := PolicyResponse{
		Actions: policy.Actions,
		Policy:  policy.Policy,
	}
	if e.matcher != nil {
		criteria := matchCriteria{constraints: policy.Policy.Spec.MatchConstraints}
		if matches, err := e.matcher.Match(&criteria, attr, namespace); err != nil {
			response.Rules = handlers.WithResponses(engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil))
			return response
		} else if !matches {
			return response
		}
	}
	results, err := policy.CompiledPolicy.Evaluate(ctx, attr, request, namespace, context)
	// TODO: error is about match conditions here ?
	if err != nil {
		response.Rules = handlers.WithResponses(engineapi.RuleError("evaluation", engineapi.Validation, "failed to load context", err, nil))
	} else {
		for index, validationResult := range results {
			ruleName := fmt.Sprintf("rule-%d", index)
			if validationResult.Error != nil {
				response.Rules = append(response.Rules, *engineapi.RuleError(ruleName, engineapi.Validation, "error", err, nil))
			} else if result, err := utils.ConvertToNative[bool](validationResult.Result); err != nil {
				response.Rules = append(response.Rules, *engineapi.RuleError(ruleName, engineapi.Validation, "conversion error", err, nil))
			} else if result {
				response.Rules = append(response.Rules, *engineapi.RulePass(ruleName, engineapi.Validation, "success", nil))
			} else {
				response.Rules = append(response.Rules, *engineapi.RuleFail(ruleName, engineapi.Validation, validationResult.Message, nil))
			}
		}
	}
	return response
}
