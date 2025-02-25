package engine

import (
	"context"
	"strings"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/autogen"
	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/cel/matching"
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
	"k8s.io/client-go/tools/cache"
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
	Policy  policiesv1alpha1.ValidatingPolicy
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

func (e *engine) matchPolicy(policy CompiledPolicy, attr admission.Attributes, namespace runtime.Object) (bool, int, error) {
	match := func(constraints *admissionregistrationv1.MatchResources) (bool, error) {
		criteria := matchCriteria{constraints: constraints}
		matches, err := e.matcher.Match(&criteria, attr, namespace)
		if err != nil {
			return false, err
		}
		return matches, nil
	}

	// match against main policy constraints
	matches, err := match(policy.Policy.Spec.MatchConstraints)
	if err != nil {
		return false, -1, err
	}
	if matches {
		return true, -1, nil
	}

	// match against autogen rules
	autogenRules := vpolautogen.ComputeRules(&policy.Policy)
	for i, autogenRule := range autogenRules {
		matches, err := match(autogenRule.MatchConstraints)
		if err != nil {
			return false, -1, err
		}
		if matches {
			return true, i, nil
		}
	}
	return false, -1, nil
}

func (e *engine) handlePolicy(ctx context.Context, policy CompiledPolicy, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context contextlib.ContextInterface) PolicyResponse {
	response := PolicyResponse{
		Actions: policy.Actions,
		Policy:  policy.Policy,
	}
	autogenIndex := -1
	if e.matcher != nil {
		matches, index, err := e.matchPolicy(policy, attr, namespace)
		if err != nil {
			response.Rules = handlers.WithResponses(engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil))
			return response
		} else if !matches {
			return response
		}
		autogenIndex = index
	}
	result, err := policy.CompiledPolicy.Evaluate(ctx, attr, request, namespace, context, autogenIndex)
	// TODO: error is about match conditions here ?
	if err != nil {
		response.Rules = handlers.WithResponses(engineapi.RuleError("evaluation", engineapi.Validation, "failed to load context", err, nil))
	} else if len(result.Exceptions) > 0 {
		var keys []string
		for i := range result.Exceptions {
			key, err := cache.MetaNamespaceKeyFunc(&result.Exceptions[i])
			if err != nil {
				response.Rules = handlers.WithResponses(engineapi.RuleError("exception", engineapi.Validation, "failed to compute exception key", err, nil))
				return response
			}
			keys = append(keys, key)
		}
		response.Rules = handlers.WithResponses(engineapi.RuleSkip("exception", engineapi.Validation, "rule is skipped due to policy exception: "+strings.Join(keys, ", "), nil).WithCELExceptions(result.Exceptions))
	} else {
		// TODO: do we want to set a rule name?
		ruleName := ""
		if result.Error != nil {
			response.Rules = append(response.Rules, *engineapi.RuleError(ruleName, engineapi.Validation, "error", err, nil))
		} else if result.Result {
			response.Rules = append(response.Rules, *engineapi.RulePass(ruleName, engineapi.Validation, "success", nil))
		} else {
			response.Rules = append(response.Rules, *engineapi.RuleFail(ruleName, engineapi.Validation, result.Message, result.AuditAnnotations))
		}
	}
	return response
}
