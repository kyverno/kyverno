package engine

import (
	"context"
	"strings"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"golang.org/x/exp/maps"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
	provider   Provider
	nsResolver NamespaceResolver
	matcher    matching.Matcher
	lister     corev1listers.SecretLister
	ivCache    imageverifycache.Client
}

func NewEngine(
	provider Provider,
	nsResolver NamespaceResolver,
	matcher matching.Matcher,
	lister corev1listers.SecretLister,
	ivCache imageverifycache.Client,
) Engine {
	return &engineImpl{
		provider:   provider,
		nsResolver: nsResolver,
		matcher:    matcher,
		lister:     lister,
		ivCache:    ivCache,
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
	responses, err := e.handleValidation(ctx, relevant, attr, &request.Request, namespace, request.Context)
	if err != nil {
		return response, err
	}
	response.Policies = append(response.Policies, responses...)
	return response, nil
}

// HandleMutating is the entry point for the ImageValidatingPolicy mutating
// admission webhook. Image verification (matching, signature/attestation
// checks, CEL validations) is a validation concern and is performed
// exclusively by HandleValidating. Running it here too would duplicate
// registry/attestor calls, and previously relied on an annotation to hand
// the outcome off to the validating webhook -- an outcome that a caller
// could forge and that would be trusted if the mutating webhook was ever
// skipped (see https://github.com/kyverno/kyverno/issues/16336).
//
// HandleMutating is therefore limited to true mutation responsibilities
// (e.g. pinning image tags to digests). No such mutation is implemented
// for ImageValidatingPolicy yet, so this returns the resource unchanged
// with no patches and no policy results.
func (e *engineImpl) HandleMutating(ctx context.Context, request EngineRequest, predicate Predicate) (eval.ImageVerifyEngineResponse, []jsonpatch.JsonPatchOperation, error) {
	var response eval.ImageVerifyEngineResponse
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, nil, err
	}
	response.Resource = &object
	if response.Resource.Object == nil {
		response.Resource = &oldObject
	}
	return response, nil, nil
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

func (e *engineImpl) handleValidation(
	ctx context.Context,
	policies []Policy,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	libctx libs.Context,
) ([]eval.ImageVerifyPolicyResponse, error) {
	responses, filteredPolicies := e.filterPolicies(policies, attr, namespace, false)
	var err error
	responses, err = e.evaluatePolicies(ctx, filteredPolicies, attr, request, namespace, libctx, request.RequestResource, responses)
	if err != nil {
		return nil, err
	}
	return maps.Values(responses), nil
}

func (e *engineImpl) filterPolicies(
	policies []Policy,
	attr admission.Attributes,
	namespace runtime.Object,
	includeUnmatched bool,
) (map[string]eval.ImageVerifyPolicyResponse, []Policy) {
	results := make(map[string]eval.ImageVerifyPolicyResponse, len(policies))
	filtered := make([]Policy, 0, len(policies))
	if e.matcher == nil {
		return results, policies
	}
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
			continue
		}
		if matches {
			filtered = append(filtered, pol)
			continue
		}
		if includeUnmatched {
			results[pol.Policy.GetName()] = response
		}
	}
	return results, filtered
}

func (e *engineImpl) evaluatePolicies(
	ctx context.Context,
	policies []Policy,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	libctx libs.Context,
	requestResource *metav1.GroupVersionResource,
	responses map[string]eval.ImageVerifyPolicyResponse,
) (map[string]eval.ImageVerifyPolicyResponse, error) {
	// leave remote and name options blank, each compiled policy will provide
	// its own credentials or the default global ones.
	ictx, err := imagedataloader.NewImageContext(e.lister, nil, nil)
	if err != nil {
		return nil, err
	}
	c := eval.NewCompiler(ictx, e.lister, requestResource, e.ivCache)
	for _, ivpol := range policies {
		response := eval.ImageVerifyPolicyResponse{
			Policy:     ivpol.Policy,
			Actions:    ivpol.Actions,
			Exceptions: ivpol.Exceptions,
		}
		startTime := time.Now()
		compiled, errList := c.Compile(ivpol.Policy, ivpol.Exceptions)
		if errList != nil {
			response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to compile policy", errList.ToAggregate(), nil)
			response.Result = response.Result.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
			responses[ivpol.Policy.GetName()] = response
			continue
		}
		result, err := compiled.Evaluate(ctx, ictx, attr, request, namespace, true, libctx)
		if err != nil {
			response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to evaluate policy", err, nil)
			response.Result = response.Result.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
			responses[ivpol.Policy.GetName()] = response
			continue
		}
		if result == nil {
			continue
		}
		if len(result.Exceptions) > 0 {
			exceptions := make([]engineapi.GenericException, 0, len(result.Exceptions))
			var keys []string
			for i := range result.Exceptions {
				key, err := cache.MetaNamespaceKeyFunc(result.Exceptions[i])
				if err != nil {
					response.Result = *engineapi.RuleError("exception", engineapi.Validation, "failed to compute exception key", err, nil)
					continue
				}
				keys = append(keys, key)
				exceptions = append(exceptions, engineapi.NewCELPolicyException(result.Exceptions[i]))
			}
			if response.Result.Name() == "" {
				response.Result = *engineapi.RuleSkip("exception", engineapi.Validation, "rule is skipped due to policy exception: "+strings.Join(keys, ", "), nil).WithExceptions(exceptions)
			}
		} else {
			ruleName := ivpol.Policy.GetName()
			if result.Error != nil {
				response.Result = *engineapi.RuleError(ruleName, engineapi.ImageVerify, "error", result.Error, nil)
			} else if result.Result {
				response.Result = *engineapi.RulePass(ruleName, engineapi.ImageVerify, "success", result.AuditAnnotations)
			} else {
				response.Result = *engineapi.RuleFail(ruleName, engineapi.ImageVerify, result.Message, result.AuditAnnotations)
			}
		}
		response.Result = response.Result.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
		responses[ivpol.Policy.GetName()] = response
	}
	return responses, nil
}
