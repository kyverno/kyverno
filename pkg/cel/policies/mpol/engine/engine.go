package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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
	if e.matcher != nil {
		constraints := mpol.Policy.GetMatchConstraints()
		if target {
			targetConstraints := mpol.Policy.GetTargetMatchConstraints()
			if len(targetConstraints.ResourceRules) > 0 {
				constraints = targetConstraints.MatchResources
			}
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
	switch {
		case mpol.ExtractionMode:
			result = e.evaluateExtractedMutation(ctx, mpol, attr, request, namespace, target)
		case target:
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

	other := oldObj
	if usingOld {
		other = newObj
	}
	otherByPath := map[string]extract.Extracted{}
	if other != nil && len(other.Object) > 0 {
		for _, t := range extract.ExtractPodTemplates(other.Object) {
			otherByPath[t.Path] = t
		}
	}

	working := source.DeepCopy()
	var mergedAudit map[string]string
	var mergedExceptions []*policiesv1beta1.PolicyException
	mutated := false

	// evaluatedAny tracks whether *any* template actually matched
	// match/targetMatchConditions and produced a real (non-nil) evaluation result
	evaluatedAny := false

	for _, tpl := range templates {
		var otherTpl *extract.Extracted
		if o, ok := otherByPath[tpl.Path]; ok {
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
		beforeBytes, err := beforeUnstr.MarshalJSON()
		if err != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: %w", tpl.Path, err)}
		}
		afterBytes, err := result.PatchedResource.MarshalJSON()
		if err != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: %w", tpl.Path, err)}
		}
		podPatch, err := jsonpatch.CreatePatch(beforeBytes, afterBytes)
		if err != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: computing patch: %w", tpl.Path, err)}
		}
		if len(podPatch) == 0 {
			continue // this template's mutation was a no-op
		}

		rebased := extract.RebasePatch(podPatch, tpl.JSONPointerPrefix())
		patched, err := applyRebasedPatch(working, rebased)
		if err != nil {
			return &compiler.EvaluationResult{Error: fmt.Errorf("pod template at %s: applying patch to parent: %w", tpl.Path, err)}
		}
		working = patched
		mutated = true

		for k, v := range result.AuditAnnotations {
			if mergedAudit == nil {
				mergedAudit = map[string]string{}
			}
			mergedAudit[k] = v
		}
	}

	// No template matched at all (every one returned nil from
	// Evaluate/EvaluateTarget) - report this as a skip, exactly like the
	// non-extraction path does when the whole policy doesn't match.
	if !evaluatedAny {
		return nil
	}

	// If nothing actually mutated but templates matched an exception, report
	// it the same way the non-extraction path does.
	if !mutated && len(mergedExceptions) > 0 {
		return &compiler.EvaluationResult{Exceptions: mergedExceptions}
	}

	return &compiler.EvaluationResult{PatchedResource: working, AuditAnnotations: mergedAudit}
}

// applyRebasedPatch applies a JSON Patch (already rebased onto the parent's
// real paths by extract.RebasePatch) to a deep copy of obj.
//
// A strict RFC-6902 apply library requires every operation's immediate
// parent path to already exist. That assumption breaks here: the
// synthesized Pod (extract.buildPod) always populates a "metadata" object,
// even when the real extracted template had none at all - so a patch
// diffed in "fake Pod space" can say "add /metadata/labels" without ever
// saying "add /metadata" first, since /metadata always existed in the fake
// Pod. Replayed onto the real object, that intermediate "metadata" key may
// genuinely be missing. This apply function auto-creates missing
// intermediate map levels as it walks the path, exactly like a normal
// patch/merge tool would.
func applyRebasedPatch(obj *unstructured.Unstructured, ops []jsonpatch.JsonPatchOperation) (*unstructured.Unstructured, error) {
	working := obj.DeepCopy()
	root := any(working.Object)
	for _, op := range ops {
		tokens, err := decodeJSONPointer(op.Path)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", op.Operation, op.Path, err)
		}
		if len(tokens) == 0 {
			continue
		}
		root, err = applyPatchOp(root, tokens, op.Operation, op.Value)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", op.Operation, op.Path, err)
		}
	}
	newRoot, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("patch replaced the document root with a non-object value (%T)", root)
	}
	working.Object = newRoot
	return working, nil
}

// applyPatchOp applies a single add/replace/remove operation at the given
// path tokens, auto-creating missing intermediate map levels (but never
// auto-creating array elements - an out-of-range array index is an error,
// since guessing array contents isn't safe). Returns the updated node;
// callers must reassign it back into their parent (maps/slices in Go don't
// let us mutate "in place" across a slice-growing append).
func applyPatchOp(node any, tokens []string, op string, value any) (any, error) {
	token, rest := tokens[0], tokens[1:]

	switch v := node.(type) {
	case map[string]any:
		if len(rest) == 0 {
			switch op {
			case "remove":
				delete(v, token)
			default: // add, replace
				v[token] = value
			}
			return v, nil
		}
		child, exists := v[token]
		if !exists || child == nil {
			if op == "remove" {
				return v, nil // nothing to remove along a path that doesn't exist
			}
			child = map[string]any{} // auto-vivify
		}
		newChild, err := applyPatchOp(child, rest, op, value)
		if err != nil {
			return nil, err
		}
		v[token] = newChild
		return v, nil

	case []any:
		idx, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("expected an array index, got %q", token)
		}
		if len(rest) == 0 {
			switch op {
			case "remove":
				if idx < 0 || idx >= len(v) {
					return v, nil
				}
				return append(v[:idx], v[idx+1:]...), nil
			case "add":
				if idx < 0 || idx > len(v) {
					return nil, fmt.Errorf("array index %d out of range (len %d)", idx, len(v))
				}
				out := make([]any, 0, len(v)+1)
				out = append(out, v[:idx]...)
				out = append(out, value)
				return append(out, v[idx:]...), nil
			default: // replace
				if idx < 0 || idx >= len(v) {
					return nil, fmt.Errorf("array index %d out of range (len %d)", idx, len(v))
				}
				v[idx] = value
				return v, nil
			}
		}
		if idx < 0 || idx >= len(v) {
			return nil, fmt.Errorf("array index %d out of range (len %d)", idx, len(v))
		}
		newChild, err := applyPatchOp(v[idx], rest, op, value)
		if err != nil {
			return nil, err
		}
		v[idx] = newChild
		return v, nil

	default:
		return nil, fmt.Errorf("cannot descend into %T at %q", node, token)
	}
}

// decodeJSONPointer splits an RFC-6901 JSON Pointer into unescaped tokens.
func decodeJSONPointer(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("invalid JSON pointer %q", path)
	}
	raw := strings.Split(path[1:], "/")
	tokens := make([]string, len(raw))
	for i, t := range raw {
		t = strings.ReplaceAll(t, "~1", "/")
		t = strings.ReplaceAll(t, "~0", "~")
		tokens[i] = t
	}
	return tokens, nil
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
