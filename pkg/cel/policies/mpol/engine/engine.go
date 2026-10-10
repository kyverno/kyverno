package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	jsonpatchv5 "github.com/evanphx/json-patch/v5"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/cel/autogen/extract"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
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
	GetCompiledPolicy(name string) (Policy, error) // todo: support namespaced as well
	GetCompiledPolicies(names ...string) map[string]Policy
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
	Policy policiesv1beta1.MutatingPolicyLike
	Rules  []engineapi.RuleResponse
}

type Predicate = func(policiesv1beta1.MutatingPolicyLike) bool

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
	mpols := e.provider.Fetch(ctx, true)
	var object *unstructured.Unstructured
	if o, ok := attr.GetObject().(*unstructured.Unstructured); ok {
		object = o
	}

	response := EngineResponse{
		Resource: object,
	}

	// The request here is loop-invariant (attr is rebuilt per policy below,
	// but the underlying admission request is not), so the `request` CEL
	// activation value is built at most once for the whole loop instead of
	// once per policy - but lazily: matching happens per policy inside
	// handlePolicy, before this is ever called, so a request matching zero
	// policies must not pay this cost at all. sync.OnceValues memoizes on
	// first actual invocation. BuildNormalizedRequestMap self-extracts
	// request.object/oldObject from request - it must not take attr's
	// (per-policy patched) object, or the hoisted map would alias mutable
	// per-policy state. If a future change makes per-target synthetic
	// requests, this hoist must be re-examined.
	requestMapFn := sync.OnceValues(func() (map[string]any, error) {
		return celcompiler.BuildNormalizedRequestMap(&request)
	})

	for _, mpol := range mpols {
		if predicate != nil && predicate(mpol.Policy) {
			r, patched := e.handlePolicy(ctx, mpol, attr, request, nil, requestMapFn, true)
			response.Policies = append(response.Policies, r)
			if patched != nil {
				response.PatchedResource = patched
				// Update attr to use the patched resource for the next policy evaluation
				attr = admission.NewAttributesRecord(
					patched,
					attr.GetOldObject(),
					attr.GetKind(),
					attr.GetNamespace(),
					attr.GetName(),
					attr.GetResource(),
					attr.GetSubresource(),
					attr.GetOperation(),
					nil,
					attr.IsDryRun(),
					attr.GetUserInfo(),
				)
			}
		}
	}
	return response, nil
}

func (e *engineImpl) Handle(ctx context.Context, request engine.EngineRequest, predicate Predicate) (EngineResponse, error) {
	var response EngineResponse
	mpols := e.provider.Fetch(ctx, false)

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
		admissionpolicy.NewUser(request.Request.UserInfo),
	)

	var namespace *corev1.Namespace
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}

	// request.Request is loop-invariant across the mpol loop below (only attr
	// is rebuilt per policy with the patched object), so the `request` CEL
	// activation value - including its object/oldObject, which derive from
	// this same original request via BuildNormalizedRequestMap's own
	// ExtractResources call (not attr's per-policy patched object) - is
	// built at most once for the whole loop, lazily: a request matching zero
	// policies below never invokes the func, so it never pays the build
	// cost. sync.OnceValues memoizes on first actual invocation.
	requestMapFn := sync.OnceValues(func() (map[string]any, error) {
		return celcompiler.BuildNormalizedRequestMap(&request.Request)
	})

	for _, mpol := range mpols {
		if predicate != nil && !predicate(mpol.Policy) {
			continue
		}
		ruleResponse, patchedResource := e.handlePolicy(ctx, mpol, attr, request.Request, namespace, requestMapFn, false)
		response.Policies = append(response.Policies, ruleResponse)
		if patchedResource != nil {
			response.PatchedResource = patchedResource
			// Update attr to use the patched resource for the next policy evaluation
			attr = admission.NewAttributesRecord(
				patchedResource,
				attr.GetOldObject(),
				attr.GetKind(),
				attr.GetNamespace(),
				attr.GetName(),
				attr.GetResource(),
				attr.GetSubresource(),
				attr.GetOperation(),
				nil,
				attr.IsDryRun(),
				attr.GetUserInfo(),
			)
		}
	}
	return response, nil
}

func (e *engineImpl) handlePolicy(ctx context.Context, mpol Policy, attr admission.Attributes, request admissionv1.AdmissionRequest, namespace *corev1.Namespace, requestMapFn func() (map[string]any, error), target bool) (MutatingPolicyResponse, *unstructured.Unstructured) {
	ruleResponse := MutatingPolicyResponse{
		Policy: mpol.Policy,
		Rules:  []engineapi.RuleResponse{},
	}

	startTime := time.Now()
	targetConstraints := mpol.Policy.GetTargetMatchConstraints()
	// A policy only defines a genuine, separate target when targetMatchConstraints is set.
	// Otherwise the resource being processed (e.g. during a background/mutateExisting scan)
	// is itself the trigger, so its matchConditions must still be evaluated - not skipped in
	// favor of the near-always-empty targetMatchConditions.
	hasExplicitTarget := len(targetConstraints.ResourceRules) > 0 || targetConstraints.Expression != ""
	if e.matcher != nil {
		constraints := mpol.Policy.GetMatchConstraints()
		if target && hasExplicitTarget {
			constraints = targetConstraints.MatchResources
		}
		matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: &constraints}, attr, namespace)
		if err != nil {
			ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
			return ruleResponse, nil
		} else if !matches {
			return ruleResponse, nil
		}
	}
	var result *compiler.EvaluationResult
	useTargetEval := target && hasExplicitTarget
	switch {
	case mpol.ExtractionMode:
		result = e.evaluateExtractedMutation(ctx, mpol, attr, request, namespace, useTargetEval)
	case useTargetEval:
		result = mpol.CompiledPolicy.EvaluateTarget(ctx, attr, namespace, request, e.typeConverter, requestMapFn, e.contextProvider)
	default:
		result = mpol.CompiledPolicy.Evaluate(ctx, attr, namespace, request, e.typeConverter, requestMapFn, e.contextProvider)
	}
	if result == nil {
		ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RuleSkip("", engineapi.Mutation, "skip", nil).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
		return ruleResponse, nil
	} else if result.Error != nil {
		ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RuleError("evaluation", engineapi.Mutation, "failed to evaluate policy", result.Error, nil).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
		return ruleResponse, nil
	} else if len(result.Exceptions) > 0 {
		exceptions := make([]engineapi.GenericException, 0, len(result.Exceptions))
		keys := make([]string, 0, len(result.Exceptions))

		var (
			highestPriority int
			selectedIndex   int
		)
		for i, ex := range result.Exceptions {
			key, err := cache.MetaNamespaceKeyFunc(ex)
			if err != nil {
				ruleResponse.Rules = handlers.WithResponses(
					engineapi.RuleError(
						"exception",
						engineapi.Mutation,
						"failed to compute exception key",
						err,
						nil,
					),
				)
				return ruleResponse, nil
			}

			keys = append(keys, key)
			exceptions = append(exceptions, engineapi.NewCELPolicyException(ex))

			// evaluate exception priority from label
			if val, ok := ex.GetLabels()[reportutils.LabelPolicyExceptionPriority]; ok {
				if p, err := strconv.Atoi(val); err == nil && p > highestPriority {
					highestPriority = p
					selectedIndex = i
				}
			}
		}
		if result.PatchedResource != nil {
			ruleResponse.Rules = append(ruleResponse.Rules,
				engineapi.RulePass("", engineapi.Mutation, "success", result.AuditAnnotations).
					WithExceptions(exceptions).
					WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
			return ruleResponse, result.PatchedResource
		}
		// determine final result based on highest-priority exception
		selectedException := result.Exceptions[selectedIndex]
		reportResult := selectedException.Spec.ReportResult

		joinedKeys := strings.Join(keys, ", ")
		msgPrefix := "rule is %s due to policy exception: " + joinedKeys
		switch reportResult {
		case string(engineapi.RuleStatusPass):
			ruleResponse.Rules = handlers.WithResponses(
				engineapi.RulePass("exception", engineapi.Mutation,
					fmt.Sprintf(msgPrefix, "passed"), nil,
				).WithExceptions(exceptions),
			)
		default:
			ruleResponse.Rules = handlers.WithResponses(
				engineapi.RuleSkip("exception", engineapi.Mutation,
					fmt.Sprintf(msgPrefix, "skipped"), nil,
				).WithExceptions(exceptions),
			)
		}
	} else {
		// Surface evaluated audit annotations as report result properties on successful evaluation.
		ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RulePass("", engineapi.Mutation, "success", result.AuditAnnotations).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
	}
	return ruleResponse, result.PatchedResource
}

// evaluateExtractedMutation implements mutation for ExtractionMode targets
// It extracts every pod-template-shaped subtree from the real admitted object,
// synthesizes a Pod from each, evaluates the same unmodified CompiledPolicy
// against each synthesized Pod, diffs the Pod before/after to get a small
// Pod-relative JSON Patch, rebases that patch onto the template's real
// location inside the parent, and applies it to a working copy of the
// real parent object. Multiple templates (e.g. JobSet's replicatedJobs[])
// accumulate onto the same working copy since their paths are disjoint.
func (e *engineImpl) evaluateExtractedMutation(ctx context.Context, mpol Policy, attr admission.Attributes, request admissionv1.AdmissionRequest, namespace *corev1.Namespace, target bool) *compiler.EvaluationResult {
	newObj, _ := attr.GetObject().(*unstructured.Unstructured)
	oldObj, _ := attr.GetOldObject().(*unstructured.Unstructured)

	source, usingOld := newObj, false
	if source == nil || len(source.Object) == 0 {
		source, usingOld = oldObj, true
	}
	if source == nil || len(source.Object) == 0 {
		return &compiler.EvaluationResult{Error: fmt.Errorf("extraction mode: expected an unstructured object, got %T", attr.GetObject())}
	}

	templates := extract.ExtractPodTemplates(source.Object)
	if len(templates) == 0 {
		return &compiler.EvaluationResult{Error: fmt.Errorf("extraction mode: no pod template found in %s/%s", source.GetAPIVersion(), source.GetKind())}
	}
	// map iteration order is random; sort so annotation merging and
	// error messages are deterministic across runs
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].JSONPointerPrefix() < templates[j].JSONPointerPrefix()
	})

	other := oldObj
	if usingOld {
		other = newObj
	}

	// Old and new templates are matched by array position.
	// Reordering entries can therefore pair the wrong templates.
	// Since extraction is schema-agnostic, we can't use list-map keys
	// (such as "name") to match them. This is a known limitation.
	otherByPath := map[string]extract.Extracted{}
	if other != nil && len(other.Object) > 0 {
		for _, t := range extract.ExtractPodTemplates(other.Object) {
			otherByPath[t.JSONPointerPrefix()] = t
		}
	}

	working := source.DeepCopy()
	var mergedAudit map[string]string
	var mergedExceptions []*policiesv1beta1.PolicyException
	var allOps []jsonpatch.JsonPatchOperation
	// evaluatedAny tracks whether *any* template actually matched
	// match/targetMatchConditions and produced a real (non-nil) evaluation result
	evaluatedAny := false

	for _, tpl := range templates {
		var otherTpl *extract.Extracted
		if o, ok := otherByPath[tpl.JSONPointerPrefix()]; ok {
			otherTpl = &o
		}

		var synthAttr admission.Attributes
		if usingOld {
			synthAttr = extract.SynthesizePodAttributes(otherTpl, &tpl, attr)
		} else {
			synthAttr = extract.SynthesizePodAttributes(&tpl, otherTpl, attr)
		}
		var synthRequest admissionv1.AdmissionRequest
		if p := extract.SynthesizePodAdmissionRequest(&request, synthAttr); p != nil {
			synthRequest = *p
		}

		var result *compiler.EvaluationResult
		if target {
			result = mpol.CompiledPolicy.EvaluateTarget(ctx, synthAttr, namespace, synthRequest, e.typeConverter, nil, e.contextProvider)
		} else {
			result = mpol.CompiledPolicy.Evaluate(ctx, synthAttr, namespace, synthRequest, e.typeConverter, nil, e.contextProvider)
		}

		if result == nil {
			continue // this template didn't match match/targetMatchConditions
		}
		evaluatedAny = true
		if result.Error != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: %w", tpl.Path, result.Error)}
		}
		if len(result.Exceptions) > 0 {
			mergedExceptions = append(mergedExceptions, result.Exceptions...)
			continue
		}
		if result.PatchedResource == nil {
			continue
		}

		beforeUnstr, ok := synthAttr.GetObject().(*unstructured.Unstructured)
		if !ok {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: expected synthesized Pod, got %T", tpl.Path, synthAttr.GetObject())}
		}

		// extract.buildPod injects placeholder metadata.name/namespace
		// (borrowed from the parent) whenever the real template declared
		// neither. Those placeholders must never appear in the diff: a
		// mutation that doesn't touch metadata would otherwise show up as
		// a spurious add/remove once rebased onto the real object. Rather
		// than guessing from the value (a deliberate mutation could
		// coincidentally choose the same value as the placeholder), strip
		// a field only when it's untouched on BOTH sides - i.e. present
		// with the exact same value before and after - since any
		// deliberate mutation changes something, even if only re-asserting
		// the same value would be indistinguishable from a no-op anyway.
		stripUntouchedPlaceholder := func(before, after *unstructured.Unstructured, field string) {
			if _, hadMetadata := tpl.Template["metadata"].(map[string]any); hadMetadata {
				if m := tpl.Template["metadata"].(map[string]any); m[field] != nil {
					return
				}
			}
			beforeVal, beforeFound, _ := unstructured.NestedString(before.Object, "metadata", field)
			afterVal, afterFound, _ := unstructured.NestedString(after.Object, "metadata", field)
			if beforeFound && afterFound && beforeVal == afterVal {
				unstructured.RemoveNestedField(before.Object, "metadata", field)
				unstructured.RemoveNestedField(after.Object, "metadata", field)
			}
		}

		beforeForDiff := beforeUnstr.DeepCopy()
		afterForDiff := result.PatchedResource.DeepCopy()
		stripUntouchedPlaceholder(beforeForDiff, afterForDiff, "name")
		stripUntouchedPlaceholder(beforeForDiff, afterForDiff, "namespace")

		beforeBytes, err := beforeForDiff.MarshalJSON()
		if err != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: %w", tpl.Path, err)}
		}
		afterBytes, err := afterForDiff.MarshalJSON()
		if err != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: %w", tpl.Path, err)}
		}
		podPatch, err := jsonpatch.CreatePatch(beforeBytes, afterBytes)
		if err != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: computing patch: %w", tpl.Path, err)}
		}

		for k, v := range result.AuditAnnotations {
			if mergedAudit == nil {
				mergedAudit = map[string]string{}
			}
			mergedAudit[k] = v
		}

		if len(podPatch) == 0 {
			continue // this template's mutation was a no-op
		}

		allOps = append(allOps, extract.RebasePatch(podPatch, tpl.JSONPointerPrefix())...)
	}

	// No template matched at all (every one returned nil from
	// Evaluate/EvaluateTarget) - report this as a skip, exactly like the
	// non-extraction path does when the whole policy doesn't match.
	if !evaluatedAny {
		return nil
	}

	// Apply every template's ops in one pass. Their paths are disjoint
	// (each sits under its own template), so this equals applying them
	// one by one but costs a single marshal/unmarshal of the parent.
	if len(allOps) > 0 {
		if err := applyRebasedPatch(working, allOps); err != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("applying patch to parent: %w", err)}
		}
	}
	mutated := len(allOps) > 0

	if !mutated && len(mergedExceptions) > 0 {
		return &compiler.EvaluationResult{Exceptions: mergedExceptions}
	}

	return &compiler.EvaluationResult{
		PatchedResource:  working,
		AuditAnnotations: mergedAudit,
		Exceptions:       mergedExceptions,
	}
}

// applyRebasedPatch applies a rebased JSON Patch to obj in place, using
// evanphx/json-patch/v5 (already vendored) instead of a hand-rolled
// applier. EnsurePathExistsOnAdd auto-creates missing intermediate map
// levels - needed since the synthesized Pod always has a "metadata" object
// even when the real template doesn't, so the diff can say "add
// /metadata/labels" without "add /metadata" first. The library's decode
// also avoids json.Number leaking into the result.
func applyRebasedPatch(obj *unstructured.Unstructured, ops []jsonpatch.JsonPatchOperation) error {
	opBytes, err := json.Marshal(ops)
	if err != nil {
		return err
	}
	patch, err := jsonpatchv5.DecodePatch(opBytes)
	if err != nil {
		return err
	}
	objBytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}
	patchedBytes, err := patch.ApplyWithOptions(objBytes, &jsonpatchv5.ApplyOptions{
		EnsurePathExistsOnAdd: true,
	})
	if err != nil {
		return err
	}
	return obj.UnmarshalJSON(patchedBytes)
}

func (e *engineImpl) GetCompiledPolicy(policyName string) (Policy, error) {
	policies := e.GetCompiledPolicies(policyName)
	if policy, ok := policies[policyName]; ok {
		return policy, nil
	}
	return Policy{}, fmt.Errorf("policy with name %s wasn't found", policyName)
}

func (e *engineImpl) GetCompiledPolicies(names ...string) map[string]Policy {
	expectedNames := make(map[string]struct{}, len(names))
	for _, name := range names {
		expectedNames[name] = struct{}{}
	}

	policies := make(map[string]Policy, len(expectedNames))
	for _, mutateExisting := range []bool{false, true} {
		compiledPolicies := e.provider.Fetch(context.TODO(), mutateExisting)
		for _, policy := range compiledPolicies {
			name := policy.Policy.GetName()
			key := PolicyKey(policy.Policy)
			if len(expectedNames) > 0 {
				// index by whichever identifier the caller used: the stable
				// policy key (namespace/name) or the bare name. Never overwrite
				// a recorded match so lookups stay stable when autogenerated
				// variants or same-named policies share an identifier.
				if _, ok := expectedNames[key]; ok {
					if _, exists := policies[key]; !exists {
						policies[key] = policy
					}
				}
				if _, ok := expectedNames[name]; ok {
					if _, exists := policies[name]; !exists {
						policies[name] = policy
					}
				}
				continue
			}
			if _, ok := policies[key]; !ok {
				policies[key] = policy
			}
			if _, ok := policies[name]; !ok {
				policies[name] = policy
			}
		}
	}
	return policies
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
		admissionpolicy.NewUser(request.Request.UserInfo),
	)

	var namespace *corev1.Namespace
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}

	// Build the `request` CEL activation value at most once for the whole
	// mutate-existing matching pass below, lazily: both provider
	// implementations (staticProvider, reconciler) call MatchesConditions
	// once per candidate policy inside their own loops, and previously each
	// call independently rebuilt the request map from scratch - the exact
	// O(policy count) cost this hoist eliminates elsewhere in mpol. A
	// request matching zero policies (or whose policies have no
	// matchConditions) never invokes this func at all.
	//
	// Reuse the object/oldObject already extracted above (for attr) via the
	// FromResources variant, so building the request map does not re-run
	// ExtractResources - a second full unmarshal of the admitted object.
	// This matching pass is read-only (attr is built once here and never
	// rebuilt with a patch, and MatchesConditions only reads), so aliasing
	// those resources into request.object/request.oldObject is safe; see
	// BuildNormalizedRequestMapFromResources' aliasing contract.
	requestMapFn := sync.OnceValues(func() (map[string]any, error) {
		return celcompiler.BuildNormalizedRequestMapFromResources(&request.Request, object, oldObject)
	})

	return e.provider.MatchesMutateExisting(ctx, attr, &request.Request, namespace, requestMapFn)
}
