package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/cel/autogen/extract"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/kyverno/pkg/logging"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"golang.org/x/exp/maps"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	provider      Provider
	nsResolver    NamespaceResolver
	matcher       matching.Matcher
	lister        corev1listers.SecretLister
	ivCache       imageverifycache.Client
	configuration config.Configuration
}

func NewEngine(
	provider Provider,
	nsResolver NamespaceResolver,
	matcher matching.Matcher,
	lister corev1listers.SecretLister,
	ivCache imageverifycache.Client,
	configuration config.Configuration,
) Engine {
	return &engineImpl{
		provider:      provider,
		nsResolver:    nsResolver,
		matcher:       matcher,
		lister:        lister,
		ivCache:       ivCache,
		configuration: configuration,
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
// HandleMutating is therefore limited to true mutation responsibilities:
// for every policy with mutateDigest enabled (the default) that matches the
// request, it pins the tag of images matched by the policy's
// matchImageReferences to their resolved digest. No signature or
// attestation verification is performed here.
//
// Digest pinning is not best effort. A policy whose digests cannot be resolved
// yields an error result rather than an error return, so the remaining policies
// still contribute their patches, but that result is surfaced by the webhook
// handler and denies the request when the policy's validationActions include
// Deny. This mirrors ClusterPolicy, where a failing handleMutateDigest appends a
// RuleError that blocks the request under failurePolicy Fail
// (pkg/engine/internal/imageverifier.go). Failing open here would silently admit
// unpinned images, which is the very problem this handler exists to fix.
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
	// fetch compiled policies
	policies, err := e.provider.Fetch(ctx)
	if err != nil {
		return response, nil, err
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
	patches, responses, err := e.handleMutation(ctx, relevant, attr, &request.Request, namespace, *response.Resource, request.Context)
	if err != nil {
		return response, nil, err
	}
	response.Policies = append(response.Policies, responses...)
	return response, patches, nil
}

func (e *engineImpl) handleMutation(
	ctx context.Context,
	policies []Policy,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	resource unstructured.Unstructured,
	libctx libs.Context,
) ([]jsonpatch.JsonPatchOperation, []eval.ImageVerifyPolicyResponse, error) {
	// leave remote and name options blank, each compiled policy will provide
	// its own credentials or the default global ones.
	ictx, err := imagedataloader.NewImageContext(e.lister, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	c := eval.NewCompiler(ictx, e.lister, request.RequestResource, imageverifycache.DisabledImageVerifyCache())

	var patches []jsonpatch.JsonPatchOperation
	var responses []eval.ImageVerifyPolicyResponse
	seen := map[string]bool{}
	for _, ivpol := range policies {
		if ivpol.ExtractionMode {
			// Digest-pinning mutation for extraction-mode targets would need
			// to write the patch back into the parent object at the
			// extracted template's path, not the top level - not
			// implemented yet. Skip rather than emit a patch at the wrong
			// location; HandleValidating still enforces the policy.
			continue
		}
		validationCfg := ivpol.Policy.GetSpec().ValidationConfigurations
		if validationCfg.MutateDigest != nil && !*validationCfg.MutateDigest {
			continue
		}
		matches, err := e.matchPolicy(ivpol, attr, namespace)
		if err != nil {
			return nil, nil, err
		}
		if !matches {
			continue
		}
		// digest mutation performs no verification, so it takes no part in the
		// request-scoped verification results
		compiled, errList := c.Compile(ivpol.Policy, ivpol.Exceptions, nil)
		if errList != nil {
			// compile errors are surfaced by the validating webhook, skip mutation
			continue
		}
		polPatches, err := compiled.MutateDigest(ctx, ictx, attr, request, namespace, resource, e.configuration)
		if err != nil {
			// Record the failure as a policy result and carry on with the remaining
			// policies rather than returning an error, which would abandon their
			// patches and deny with an unattributed message. This mirrors
			// ClusterPolicy, where a failing handleMutateDigest appends a RuleError
			// and continues (pkg/engine/internal/imageverifier.go). The webhook
			// handler turns this result into a denial when the policy's
			// validationActions include Deny.
			logging.WithName("ivpol/mutateDigest").Error(err, "failed to update digest",
				"policy", ivpol.Policy.GetName())
			responses = append(responses, eval.ImageVerifyPolicyResponse{
				Policy:     ivpol.Policy,
				Actions:    ivpol.Actions,
				Exceptions: ivpol.Exceptions,
				Result:     *engineapi.RuleError("mutateDigest", engineapi.ImageVerify, "failed to update digest", err, nil),
			})
			// deliberately no continue: MutateDigest resolves each image on its own, so
			// the images it did pin keep their patch even though another one failed
		}
		for _, p := range polPatches {
			if seen[p.Path] {
				continue
			}
			seen[p.Path] = true
			patches = append(patches, p)
		}
	}
	return patches, responses, nil
}

// evaluateExtractedIv mirrors vpol's evaluateExtracted: instead of evaluating
// compiled against the real admitted object (a custom workload CRD, whose
// shape the policy's Pod-targeted rule knows nothing about), it extracts
// every pod-template-shaped subtree, synthesizes a Pod from each, and
// evaluates the same unmodified compiled policy against each synthesized Pod
// in turn. Any failing/erroring template denies the whole request; a
// resource with no discoverable pod template is an explicit error rather
// than a silent pass.
func (e *engineImpl) evaluateExtractedIv(
	ctx context.Context,
	compiled eval.CompiledPolicy,
	ictx imagedataloader.ImageContext,
	attr admission.Attributes,
	request interface{},
	namespace runtime.Object,
	libctx libs.Context,
) (*eval.EvaluationResult, error) {
	obj, ok := attr.GetObject().(*unstructured.Unstructured)
	if !ok || obj == nil {
		return &eval.EvaluationResult{Error: fmt.Errorf("extraction mode: expected an unstructured object, got %T", attr.GetObject())}, nil
	}
	templates := extract.ExtractPodTemplates(obj.Object)
	if len(templates) == 0 {
		return &eval.EvaluationResult{Error: fmt.Errorf("extraction mode: no pod template found in %s/%s", obj.GetAPIVersion(), obj.GetKind())}, nil
	}
	var oldByPath map[string]extract.Extracted
	if oldObj, ok := attr.GetOldObject().(*unstructured.Unstructured); ok && oldObj != nil {
		oldTemplates := extract.ExtractPodTemplates(oldObj.Object)
		oldByPath = make(map[string]extract.Extracted, len(oldTemplates))
		for _, t := range oldTemplates {
			oldByPath[t.Path] = t
		}
	}
	var last *eval.EvaluationResult
	for _, tpl := range templates {
		var oldTpl *extract.Extracted
		if o, ok := oldByPath[tpl.Path]; ok {
			oldTpl = &o
		}
		synthAttr := extract.SynthesizePodAttributes(tpl, oldTpl, attr)
		result, err := compiled.Evaluate(ctx, ictx, synthAttr, request, namespace, true, libctx)
		if err != nil {
			return nil, fmt.Errorf("pod template at %s: %w", tpl.Path, err)
		}
		if result != nil && (result.Error != nil || !result.Result) {
			if result.Message != "" {
				result.Message = fmt.Sprintf("%s (pod template at %s)", result.Message, tpl.Path)
			}
			return result, nil
		}
		last = result
	}
	return last, nil
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
	// shared by every policy compiled below, so required sees cross-policy evidence
	verifications := imageverify.NewImageVerificationResults()
	// resolved after the loop: evidence may come from a policy evaluated later
	var pendingRequired []pendingRequiredCheck
	for _, ivpol := range policies {
		response := eval.ImageVerifyPolicyResponse{
			Policy:     ivpol.Policy,
			Actions:    ivpol.Actions,
			Exceptions: ivpol.Exceptions,
		}
		startTime := time.Now()
		compiled, errList := c.Compile(ivpol.Policy, ivpol.Exceptions, verifications)
		if errList != nil {
			response.Result = *engineapi.RuleError("evaluation", engineapi.ImageVerify, "failed to compile policy", errList.ToAggregate(), nil)
			response.Result = response.Result.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
			responses[ivpol.Policy.GetName()] = response
			continue
		}
		var result *eval.EvaluationResult
		if ivpol.ExtractionMode {
			result, err = e.evaluateExtractedIv(ctx, compiled, ictx, attr, request, namespace, libctx)
		} else {
			result, err = compiled.Evaluate(ctx, ictx, attr, request, namespace, true, libctx)
		}
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
				pendingRequired = append(pendingRequired, pendingRequiredCheck{
					name:             ivpol.Policy.GetName(),
					compiled:         compiled,
					images:           result.MatchedImages,
					auditAnnotations: result.AuditAnnotations,
					startTime:        startTime,
				})
			} else {
				response.Result = *engineapi.RuleFail(ruleName, engineapi.ImageVerify, result.Message, result.AuditAnnotations)
			}
		}
		response.Result = response.Result.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
		responses[ivpol.Policy.GetName()] = response
	}
	enforceRequired(pendingRequired, responses)
	return responses, nil
}

// pendingRequiredCheck defers a passing policy's required check until every
// policy in the request has contributed its verifications.
type pendingRequiredCheck struct {
	name             string
	compiled         eval.CompiledPolicy
	images           []string
	auditAnnotations map[string]string
	startTime        time.Time
}

// enforceRequired turns a policy that passed its validations into a failure when
// one of the images it matched was never verified, by any policy in the request.
func enforceRequired(checks []pendingRequiredCheck, responses map[string]eval.ImageVerifyPolicyResponse) {
	for _, check := range checks {
		err := check.compiled.EnforceRequired(check.images)
		if err == nil {
			continue
		}
		response, ok := responses[check.name]
		if !ok {
			continue
		}
		result := *engineapi.RuleFail(check.name, engineapi.ImageVerify, err.Error(), check.auditAnnotations)
		response.Result = result.WithStats(engineapi.NewExecutionStats(check.startTime, time.Now()))
		responses[check.name] = response
	}
}
