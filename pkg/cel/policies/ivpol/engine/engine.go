package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/kyverno/pkg/logging"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"golang.org/x/exp/maps"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
)

type (
	EngineRequest  = engine.EngineRequest
	EngineResponse = engine.EngineResponse
	Predicate      = func(policiesv1beta1.ImageValidatingPolicyLike) bool
)

type Engine interface {
	HandleMutating(context.Context, EngineRequest, Predicate) (eval.ImageVerifyEngineResponse, []jsonpatch.JsonPatchOperation, error)
	HandleValidating(context.Context, EngineRequest, Predicate) (eval.ImageVerifyEngineResponse, error)
}

type NamespaceResolver = engine.NamespaceResolver

type engineImpl struct {
	provider     Provider
	nsResolver   NamespaceResolver
	matcher      matching.Matcher
	lister       k8scorev1.SecretInterface
	registryOpts []imagedataloader.Option
}

func NewEngine(
	provider Provider,
	nsResolver NamespaceResolver,
	matcher matching.Matcher,
	lister k8scorev1.SecretInterface,
	registryOpts []imagedataloader.Option,
) Engine {
	return &engineImpl{
		provider:     provider,
		nsResolver:   nsResolver,
		matcher:      matcher,
		lister:       lister,
		registryOpts: registryOpts,
	}
}

func (e *engineImpl) HandleValidating(ctx context.Context, request EngineRequest, predicate Predicate) (eval.ImageVerifyEngineResponse, error) {
	var response eval.ImageVerifyEngineResponse
	// fetch compiled policies
	policies, err := e.provider.Fetch(ctx)
	if err != nil {
		return response, err
	}
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, err
	}
	response.Resource = &object
	if response.Resource.Object == nil {
		response.Resource = &oldObject
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
		admissionpolicy.NewUser(request.Request.UserInfo),
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}
	// evaluate policies
	var relevant []Policy
	if predicate != nil {
		for _, policy := range policies {
			if !predicate(policy.Policy) {
				continue
			}
			relevant = append(relevant, policy)
		}
	} else {
		relevant = policies
	}
	responses, err := e.handleValidation(relevant, attr, namespace)
	if err != nil {
		return response, err
	}
	response.Policies = append(response.Policies, responses...)
	return response, nil
}

func (e *engineImpl) HandleMutating(ctx context.Context, request EngineRequest, predicate Predicate) (eval.ImageVerifyEngineResponse, []jsonpatch.JsonPatchOperation, error) {
	var response eval.ImageVerifyEngineResponse
	// fetch compiled policies
	policies, err := e.provider.Fetch(ctx)
	if err != nil {
		return response, nil, err
	}
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, nil, err
	}
	response.Resource = &object
	if response.Resource.Object == nil {
		response.Resource = &oldObject
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
		admissionpolicy.NewUser(request.Request.UserInfo),
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}
	// evaluate policies
	var relevant []Policy
	if predicate != nil {
		for _, policy := range policies {
			if !predicate(policy.Policy) {
				continue
			}
			relevant = append(relevant, policy)
		}
	} else {
		relevant = policies
	}
	responses, patches, err := e.handleMutation(ctx, relevant, attr, &request.Request, namespace, request.Context)
	if err != nil {
		return response, nil, err
	}
	response.Policies = append(response.Policies, responses...)
	return response, patches, nil
}

func (e *engineImpl) matchPolicy(policy Policy, attr admission.Attributes, namespace runtime.Object) (bool, error) {
	match := func(constraints *admissionregistrationv1.MatchResources) (bool, error) {
		criteria := matching.MatchCriteria{Constraints: constraints}
		matches, err := e.matcher.Match(&criteria, attr, namespace)
		if err != nil {
			return false, err
		}
		return matches, nil
	}
	// match against main policy constraints
	matches, err := match(policy.Policy.GetSpec().MatchConstraints)
	if err != nil {
		return false, err
	}
	if matches {
		return true, nil
	}
	return false, nil
}

func (e *engineImpl) handleMutation(
	ctx context.Context,
	policies []Policy,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
) ([]eval.ImageVerifyPolicyResponse, []jsonpatch.JsonPatchOperation, error) {
	results := make(map[string]eval.ImageVerifyPolicyResponse, len(policies))
	filteredPolicies := make([]Policy, 0)
	if e.matcher != nil {
		for _, pol := range policies {
			matches, err := e.matchPolicy(pol, attr, namespace)
			response := eval.ImageVerifyPolicyResponse{
				Policy:     pol.Policy,
				Actions:    pol.Actions,
				Exceptions: pol.Exceptions,
			}
			if err != nil {
				response.Result = *engineapi.RuleError("match", engineapi.ImageVerify, "failed to execute matching", err, nil)
				results[pol.Policy.GetName()] = response
			} else if matches {
				filteredPolicies = append(filteredPolicies, pol)
			} else {
				if !matches {
					results[pol.Policy.GetName()] = response
				}
			}
		}
	}
	logger := logging.GlobalLogger().WithName("ivpol-engine")
	ictx, err := imagedataloader.NewImageContext(e.lister, e.registryOpts...)
	if err != nil {
		return nil, nil, err
	}
	c := eval.NewCompiler(ictx, e.lister, request.RequestResource)
	logger.V(4).Info("evaluating ImageValidatingPolicies (mutating phase)", "policies", len(filteredPolicies), "resource", attr.GetName(), "namespace", attr.GetNamespace())
	for _, ivpol := range filteredPolicies {
		response := eval.ImageVerifyPolicyResponse{
			Policy:     ivpol.Policy,
			Actions:    ivpol.Actions,
			Exceptions: ivpol.Exceptions,
		}
		startTime := time.Now()
		logger.V(4).Info("compiling ImageValidatingPolicy", "policy", ivpol.Policy.GetName())
		if p, errList := c.Compile(ivpol.Policy, ivpol.Exceptions); errList != nil {
			logger.V(4).Info("failed to compile ImageValidatingPolicy", "policy", ivpol.Policy.GetName(), "error", errList.ToAggregate())
			response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to compile policy", errList.ToAggregate(), nil)
		} else {
			logger.V(4).Info("evaluating ImageValidatingPolicy", "policy", ivpol.Policy.GetName())
			result, err := p.Evaluate(ctx, ictx, attr, request, namespace, true, context)
			if err != nil {
				logger.V(4).Info("ImageValidatingPolicy evaluation error", "policy", ivpol.Policy.GetName(), "error", err)
				response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to evaluate policy", err, nil)
				results[ivpol.Policy.GetName()] = response
			} else if result != nil {
				if len(result.Exceptions) > 0 {
					exceptions := make([]engineapi.GenericException, 0, len(result.Exceptions))
					var keys []string
					for i := range result.Exceptions {
						key, err := cache.MetaNamespaceKeyFunc(&result.Exceptions[i])
						if err != nil {
							response.Result = *engineapi.RuleError("exception", engineapi.Validation, "failed to compute exception key", err, nil)
						}
						keys = append(keys, key)
						exceptions = append(exceptions, engineapi.NewCELPolicyException(result.Exceptions[i]))
					}
					logger.V(4).Info("ImageValidatingPolicy skipped due to exception", "policy", ivpol.Policy.GetName(), "exceptions", keys)
					response.Result = *engineapi.RuleSkip("exception", engineapi.Validation, "rule is skipped due to policy exception: "+strings.Join(keys, ", "), nil).WithExceptions(exceptions)
				} else {
					ruleName := ivpol.Policy.GetName()
					if result.Error != nil {
						logger.V(4).Info("ImageValidatingPolicy evaluation error", "policy", ruleName, "error", result.Error)
						response.Result = *engineapi.RuleError(ruleName, engineapi.ImageVerify, "error", result.Error, nil)
					} else if result.Result {
						logger.V(4).Info("ImageValidatingPolicy evaluation passed", "policy", ruleName)
						response.Result = *engineapi.RulePass(ruleName, engineapi.ImageVerify, "success", result.AuditAnnotations)
					} else {
						logger.V(4).Info("ImageValidatingPolicy evaluation failed", "policy", ruleName, "message", result.Message)
						response.Result = *engineapi.RuleFail(ruleName, engineapi.ImageVerify, result.Message, result.AuditAnnotations)
					}
				}

				results[ivpol.Policy.GetName()] = response
			}
		}
		response.Result = response.Result.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
	}
	ann, err := objectAnnotations(attr)
	if err != nil {
		return nil, nil, err
	}
	patches, err := eval.MakeImageVerifyOutcomePatch(len(ann) != 0, results)
	if err != nil {
		return nil, nil, err
	}
	return maps.Values(results), patches, nil
}

func objectAnnotations(attr admission.Attributes) (map[string]string, error) {
	obj := attr.GetObject()
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return nil, nil
	}
	ret, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: ret}
	return u.GetAnnotations(), nil
}

func (e *engineImpl) handleValidation(
	policies []Policy,
	attr admission.Attributes,
	namespace runtime.Object,
) ([]eval.ImageVerifyPolicyResponse, error) {
	logger := logging.GlobalLogger().WithName("ivpol-engine")
	responses := make(map[string]eval.ImageVerifyPolicyResponse)
	annotations, err := objectAnnotations(attr)
	if err != nil {
		return nil, err
	}
	if len(annotations) == 0 {
		return nil, fmt.Errorf("annotations not present on object, image verification failed")
	}
	filteredPolicies := make([]Policy, 0)
	if e.matcher != nil {
		for _, pol := range policies {
			matches, err := e.matchPolicy(pol, attr, namespace)
			response := eval.ImageVerifyPolicyResponse{
				Policy:     pol.Policy,
				Actions:    pol.Actions,
				Exceptions: pol.Exceptions,
			}
			if err != nil {
				response.Result = *engineapi.RuleError("match", engineapi.ImageVerify, "failed to execute matching", err, nil)
				responses[pol.Policy.GetName()] = response
			} else if matches {
				logger.V(4).Info("ImageValidatingPolicy matched resource (validating phase)", "policy", pol.Policy.GetName(), "resource", attr.GetName(), "namespace", attr.GetNamespace())
				filteredPolicies = append(filteredPolicies, pol)
			} else {
				logger.V(6).Info("ImageValidatingPolicy did not match resource", "policy", pol.Policy.GetName(), "resource", attr.GetName(), "namespace", attr.GetNamespace())
			}
		}
	}
	if data, found := annotations[kyverno.AnnotationImageVerifyOutcomes]; !found {
		return nil, fmt.Errorf("%s annotation not present", kyverno.AnnotationImageVerifyOutcomes)
	} else {
		var outcomes map[string]eval.ImageVerificationOutcome
		if err := json.Unmarshal([]byte(data), &outcomes); err != nil {
			return nil, err
		}
		for _, pol := range filteredPolicies {
			resp := eval.ImageVerifyPolicyResponse{
				Policy:  pol.Policy,
				Actions: pol.Actions,
			}
			startTime := time.Now()
			if o, found := outcomes[pol.Policy.GetName()]; !found {
				logger.V(4).Info("ImageValidatingPolicy outcome not found in annotation", "policy", pol.Policy.GetName())
				resp.Result = *engineapi.RuleFail(pol.Policy.GetName(), engineapi.ImageVerify, "policy not evaluated", nil)
			} else {
				logger.V(4).Info("ImageValidatingPolicy outcome", "policy", pol.Policy.GetName(), "status", o.Status, "message", o.Message)
				resp.Result = *engineapi.NewRuleResponse(o.Name, engineapi.ImageVerify, o.Message, o.Status, o.Properties)
			}
			resp.Result = resp.Result.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
			responses[pol.Policy.GetName()] = resp
		}
	}
	return maps.Values(responses), nil
}
