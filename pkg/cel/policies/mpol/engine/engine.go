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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/tools/cache"
)

// Engine evaluates MutatingPolicies against admission requests and existing
// resources. It is the central entry point for both the admission webhook path
// (Handle) and the background/CLI mutate-existing path (Evaluate).
type Engine interface {
	// Handle evaluates all non-mutateExisting policies against the given
	// admission request and applies any matching mutations.
	Handle(context.Context, engine.EngineRequest, Predicate) (EngineResponse, error)
	// Evaluate evaluates mutateExisting policies against the supplied target
	// resource, resolving its namespace so that namespaceSelector matching
	// mirrors the admission path.
	Evaluate(context.Context, admission.Attributes, admissionv1.AdmissionRequest, Predicate) (EngineResponse, error)
	// MatchedMutateExistingPolicies returns the names of all mutateExisting
	// policies whose trigger constraints match the given admission request.
	MatchedMutateExistingPolicies(context.Context, engine.EngineRequest) []string
	// GetCompiledPolicy returns the compiled representation of the policy
	// identified by name. Only cluster-scoped names are supported for now.
	GetCompiledPolicy(name string) (Policy, error) // todo: support namespaced as well
	// GetCompiledPolicies returns compiled policies indexed by the provided
	// names. When names is empty all compiled policies are returned.
	GetCompiledPolicies(names ...string) map[string]Policy
}

// EngineResponse is the result of evaluating one or more MutatingPolicies
// against a resource. PatchedResource is non-nil only when at least one
// policy produced a mutation.
type EngineResponse struct {
	PatchedResource *unstructured.Unstructured
	Resource        *unstructured.Unstructured
	Policies        []MutatingPolicyResponse
}

// GetPatches computes the JSON Patch operations needed to transform Resource
// into PatchedResource. It returns nil if either resource is missing or the
// diff cannot be computed.
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

// MutatingPolicyResponse holds the per-policy result of a single engine
// evaluation pass, including every rule response produced for that policy.
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

// NewEngine constructs an Engine backed by the given provider and helpers.
// Pass matching.NewMatcher() for matcher in production; nil disables
// matchConstraints/namespaceSelector evaluation (for testing only).
func NewEngine(provider Provider, nsResolver engine.NamespaceResolver, matcher matching.Matcher, typeConverter compiler.TypeConverterManager, contextProvider libs.Context) *engineImpl {
	return &engineImpl{
		provider:        provider,
		nsResolver:      nsResolver,
		matcher:         matcher,
		typeConverter:   typeConverter,
		contextProvider: contextProvider,
	}
}

// Evaluate processes mutateExisting MutatingPolicies for a target resource
// identified by attr. The namespace of attr is resolved via nsResolver so
// that namespaceSelector matching mirrors the admission-webhook path. A nil
// predicate evaluates all policies.
func (e *engineImpl) Evaluate(ctx context.Context, attr admission.Attributes, request admissionv1.AdmissionRequest, predicate Predicate) (EngineResponse, error) {
	mpols := e.provider.Fetch(ctx, true)
	var object *unstructured.Unstructured
	if o, ok := attr.GetObject().(*unstructured.Unstructured); ok {
		object = o
	}

	response := EngineResponse{
		Resource: object,
	}

	// Resolve the namespace so that namespaceSelector matching in handlePolicy
	// mirrors the admission path (Handle), which also calls nsResolver.
	var namespace *corev1.Namespace
	if ns := attr.GetNamespace(); ns != "" {
		namespace = e.nsResolver(ns)
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
		if predicate != nil && !predicate(mpol.Policy) {
			continue
		}
		r, patched := e.handlePolicy(ctx, mpol, attr, request, namespace, requestMapFn, true)
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
	return response, nil
}

// Handle evaluates admission-time (non-mutateExisting) MutatingPolicies
// against request. Policies that pass the predicate gate are matched and,
// when applicable, their mutations are applied. A nil predicate evaluates
// all policies.
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

// handlePolicy runs a single compiled MutatingPolicy against attr. When
// target is true the policy is evaluated in target mode (mutateExisting):
// constraints are taken from targetMatchConstraints and target operations are
// normalised to OperationAll so that the background controller's synthetic
// Update and the CLI's empty operation do not incorrectly filter out targets.
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
		if target {
			if hasExplicitTarget {
				if len(targetConstraints.ResourceRules) > 0 {
					constraints = targetConstraints.MatchResources
				} else if targetConstraints.Expression != "" {
					// Expression-only targets: the CEL expression (e.g. resource.get(...))
					// resolves the target set directly. Use the targetMatchConstraints'
					// MatchResources (which may carry NamespaceSelector/ObjectSelector)
					// but clear ResourceRules to avoid falling
					// back to the trigger's matchConstraints for resource-rule matching.
					// We preserve ExcludeResourceRules so exclusion logic still applies.
					constraints = targetConstraints.MatchResources
					constraints.ResourceRules = nil
				}
			}

			// Normalize target resource-rule operations to OperationAll.
			// Target matching is not an admission-operation filter: the
			// background controller synthesizes Update for every scan request,
			// and the CLI uses an empty operation. Without this normalization,
			// a target rule limited to CREATE would be silently rejected even
			// though the target was already selected for mutation.
			if len(constraints.ResourceRules) > 0 {
				normalizedRules := make([]admissionregistrationv1.NamedRuleWithOperations, len(constraints.ResourceRules))
				for i, r := range constraints.ResourceRules {
					normalizedRules[i] = r
					normalizedRules[i].Operations = []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}
				}
				constraints.ResourceRules = normalizedRules
			}
			if len(constraints.ExcludeResourceRules) > 0 {
				normalizedExclude := make([]admissionregistrationv1.NamedRuleWithOperations, len(constraints.ExcludeResourceRules))
				for i, r := range constraints.ExcludeResourceRules {
					normalizedExclude[i] = r
					normalizedExclude[i].Operations = []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}
				}
				constraints.ExcludeResourceRules = normalizedExclude
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
	if mpol.ExtractionMode {
		// Mutating a custom workload CRD correctly requires writing the
		// patch back into the parent object at the extracted template's
		// path, not the top level - not implemented yet. Skip rather than
		// apply the policy's Pod-shaped ApplyConfiguration to the literal
		// admitted object, which would produce a meaningless or broken
		// patch. ValidatingPolicy/ImageValidatingPolicy extraction-mode
		// targets are unaffected - this only concerns mutation.
		ruleResponse.Rules = append(ruleResponse.Rules, engineapi.RuleSkip("", engineapi.Mutation, "extraction mode: mutation for custom workload CRDs is not yet supported", nil).WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
		return ruleResponse, nil
	}
	var result *compiler.EvaluationResult
	if target && hasExplicitTarget {
		result = mpol.CompiledPolicy.EvaluateTarget(ctx, attr, namespace, request, e.typeConverter, requestMapFn, e.contextProvider)
	} else {
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

// GetCompiledPolicy returns the compiled Policy for the given name. It
// searches across both admission and mutateExisting policy sets and returns
// an error if the policy is not found.
func (e *engineImpl) GetCompiledPolicy(policyName string) (Policy, error) {
	policies := e.GetCompiledPolicies(policyName)
	if policy, ok := policies[policyName]; ok {
		return policy, nil
	}
	return Policy{}, fmt.Errorf("policy with name %s wasn't found", policyName)
}

// GetCompiledPolicies returns compiled policies indexed by each provided name.
// Policies are looked up by both their stable policy key (namespace/name) and
// their bare name. When names is empty all compiled policies are returned.
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

// MatchedMutateExistingPolicies returns the policy names (or namespace/name
// keys) of all mutateExisting MutatingPolicies whose trigger constraints
// match request. Used by the background controller to determine which
// policies must be re-evaluated for a given admission event.
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
