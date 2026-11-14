// Package engine evaluates AuthorizingPolicy resources for the Kubernetes
// conditional authorization protocol (KEP-5681, Kubernetes v1.38 alpha).
//
// The evaluator is order-independent and phase-aware:
//
//   - Each request reads one immutable Snapshot of compiled policies.
//     Policies are compiled and validated at reconcile time; a policy that
//     failed to compile stays in the snapshot and its rules follow the
//     effect-specific error semantics (it never silently exempts a request).
//   - At SubjectAccessReview time, `request` and `variables` are bound and
//     `object`/`oldObject` are unknown (cel-go partial evaluation with
//     unknown attribute patterns, state tracking, and residual ASTs).
//   - Every rule of every applicable policy is collected into one flat set and
//     folded with the precedence Deny > NoOpinion > Allow. Only concrete
//     outcomes are folded; residuals stay conditional.
//   - Residual conditions are self-contained: known values are substituted by
//     pruning, and any remaining `request`/`variables` reference is captured
//     with `cel.bind`. The AuthorizationConditionsReview callback evaluates the
//     residuals carried in the decision and never reads current policies.
//   - Input size, residual size, CEL cost, value depth, and concurrency are
//     bounded, and context cancellation stops evaluation (fail closed).
package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/kyverno/kyverno/pkg/logging"
	admissionv1 "k8s.io/api/admission/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/cel/lazy"
	"k8s.io/utils/lru"
)

var logger = logging.WithName("apol-engine")

const (
	// ConditionType marks conditions Kyverno authored and evaluates itself.
	// It also versions the conditions environment (ConditionsEnvVersion): a
	// residual of this type is a CEL expression over `object` and `oldObject`
	// only, using the pure Kubernetes CEL libraries plus `cel.bind`. An
	// incompatible change to that environment must use a new condition type,
	// so a replica never evaluates a cached decision with the wrong semantics;
	// unknown types evaluate as errors (fail closed).
	//
	// The kube-apiserver type `authorizer.kubernetes.io/cel` is not used:
	// kube-apiserver v0.38.0-beta.0 has no built-in evaluator for it.
	ConditionType = "policies.kyverno.io/cel"
	// ConditionsEnvVersion is the version of the residual condition environment
	// identified by ConditionType. It is recorded for diagnostics.
	ConditionsEnvVersion = "v1"
	// MaxConditionsPerMap mirrors the upstream ConditionsMap limit (total over all effects).
	MaxConditionsPerMap = authorizer.MaxConditionsPerMap
	// definite effect condition IDs (constant `true` conditions). Rule names
	// are DNS labels, which cannot contain `_`, so these IDs never collide
	// with a `<policy>/<rule>` ID, even for a policy named policies.kyverno.io.
	definiteNoOpinionID = "policies.kyverno.io/definite_noopinion"
	definiteAllowID     = "policies.kyverno.io/definite_allow"
)

// conditionalVerbs are the verbs that can carry a conditional decision.
var conditionalVerbs = sets.New("create", "update", "patch", "delete")

// Limits bound the work the engine does per request.
type Limits struct {
	// MaxRequestBytes bounds the JSON size of a SubjectAccessReview spec.
	MaxRequestBytes int
	// MaxObjectBytes bounds the combined raw size of object and oldObject in a
	// conditions review.
	MaxObjectBytes int
	// MaxValueDepth bounds the nesting depth of decoded objects and captured values.
	MaxValueDepth int
	// TotalCostLimit bounds the CEL cost of all rules (and variables) of one
	// SubjectAccessReview, and of all conditions of one conditions review.
	// Each expression is also bounded by celconfig.PerCallLimit.
	TotalCostLimit uint64
	// MaxConditionBytes bounds one residual condition (upstream limit).
	MaxConditionBytes int
	// MaxConcurrent bounds concurrent evaluations of each kind: SubjectAccessReviews
	// and conditions reviews have separate pools of this size, so one kind
	// cannot starve the other. Callers wait for a slot until their context is done.
	MaxConcurrent int
	// ConditionCacheSize bounds the cache of compiled condition programs.
	ConditionCacheSize int
}

// DefaultLimits returns the default limits.
func DefaultLimits() Limits {
	return Limits{
		MaxRequestBytes:    64 * 1024,
		MaxObjectBytes:     8 * 1024 * 1024,
		MaxValueDepth:      64,
		TotalCostLimit:     celconfig.RuntimeCELCostBudget,
		MaxConditionBytes:  authorizer.MaxConditionBytes,
		MaxConcurrent:      256,
		ConditionCacheSize: 4096,
	}
}

// Option configures an engine.
type Option func(*engine)

// WithLimits overrides the default limits. Zero fields keep their default.
func WithLimits(l Limits) Option {
	return func(e *engine) {
		d := DefaultLimits()
		if l.MaxRequestBytes > 0 {
			d.MaxRequestBytes = l.MaxRequestBytes
		}
		if l.MaxObjectBytes > 0 {
			d.MaxObjectBytes = l.MaxObjectBytes
		}
		if l.MaxValueDepth > 0 {
			d.MaxValueDepth = l.MaxValueDepth
		}
		if l.TotalCostLimit > 0 {
			d.TotalCostLimit = l.TotalCostLimit
		}
		if l.MaxConditionBytes > 0 && l.MaxConditionBytes < d.MaxConditionBytes {
			d.MaxConditionBytes = l.MaxConditionBytes
		}
		if l.MaxConcurrent > 0 {
			d.MaxConcurrent = l.MaxConcurrent
		}
		if l.ConditionCacheSize > 0 {
			d.ConditionCacheSize = l.ConditionCacheSize
		}
		e.limits = d
	}
}

// CompiledPolicy is one AuthorizingPolicy after compilation.
type CompiledPolicy interface {
	Policy() *policiesv1alpha1.AuthorizingPolicy
	// Applies evaluates matchSubjects and matchResources (no CEL).
	Applies(spec *authorizationv1.SubjectAccessReviewSpec) bool
	// CompileError is non-nil when the policy failed to compile; rules then follow the error semantics.
	CompileError() error
}

// Engine is the evaluator boundary used by the SAR and conditions-review handlers.
type Engine interface {
	// Authorize computes the authorization-time decision. conditionsAware is true only when
	// spec.authorizationOptions.handledDecisionTypes contains ConditionsMap (and Union). The
	// returned decision type is Allow, Deny, NoOpinion, or ConditionsMap (never Union).
	Authorize(ctx context.Context, spec *authorizationv1.SubjectAccessReviewSpec, conditionsAware bool) authorizationv1.ConditionsAwareDecision
	// EvaluateConditions evaluates a decision snapshot previously returned by Authorize against
	// admission data. It ignores current policy state. The returned type is Allow, Deny, or NoOpinion.
	EvaluateConditions(ctx context.Context, decision *authorizationv1.ConditionsAwareDecision, admission *admissionv1.AdmissionRequest) authorizationv1.ConditionsAwareDecision
}

// NewCompiledPolicy compiles a policy and wraps the result. Call it at
// reconcile time, never on the request path.
func NewCompiledPolicy(c apolcompiler.Compiler, policy *policiesv1alpha1.AuthorizingPolicy) CompiledPolicy {
	compiled, errs := c.Compile(policy)
	cp := &compiledPolicy{policy: policy, compiled: compiled}
	if len(errs) > 0 {
		cp.err = errs.ToAggregate()
	}
	return cp
}

type compiledPolicy struct {
	policy   *policiesv1alpha1.AuthorizingPolicy
	compiled *apolcompiler.Policy
	err      error
}

func (p *compiledPolicy) Policy() *policiesv1alpha1.AuthorizingPolicy { return p.policy }
func (p *compiledPolicy) CompileError() error                         { return p.err }

func (p *compiledPolicy) Applies(spec *authorizationv1.SubjectAccessReviewSpec) bool {
	if spec == nil {
		return false
	}
	if !p.policy.Spec.MatchesSubject(policiesv1alpha1.AuthorizationSubject{User: spec.User, Groups: spec.Groups}) {
		return false
	}
	switch {
	case spec.ResourceAttributes != nil:
		ra := spec.ResourceAttributes
		return p.policy.Spec.MatchResources.MatchesResource(policiesv1alpha1.AuthorizationResourceAttributes{
			APIGroup:    ra.Group,
			APIVersion:  ra.Version,
			Resource:    ra.Resource,
			Subresource: ra.Subresource,
			Namespace:   ra.Namespace,
			Name:        ra.Name,
			Verb:        ra.Verb,
		})
	case spec.NonResourceAttributes != nil:
		nra := spec.NonResourceAttributes
		return p.policy.Spec.MatchResources.MatchesNonResource(policiesv1alpha1.AuthorizationNonResourceAttributes{
			Path: nra.Path,
			Verb: nra.Verb,
		})
	}
	return false
}

type engine struct {
	provider Provider
	limits   Limits
	// authorize serves the SubjectAccessReview path (residual self-checks);
	// review serves the AuthorizationConditionsReview path, whose conditions
	// come from the caller. Each has its own environment, compile lock,
	// program cache, and concurrency slots, so conditions reviews cannot block
	// or evict the SubjectAccessReview path.
	authorize *conditionsPool
	review    *conditionsPool
}

// conditionsPool compiles and caches condition programs in one conditions
// environment and bounds concurrent evaluations.
type conditionsPool struct {
	env      *cel.Env
	sem      chan struct{}
	programs *lru.Cache
	// mu serializes type-checking on the shared environment. cel.dev/cel-go
	// v0.33.2 has a data race in checker.(*Env).Catalog when a shared env
	// reports an undeclared reference concurrently (seen with go test -race).
	// Compiled programs are cached, so the lock is held only on cache misses.
	mu sync.Mutex
}

func newConditionsPool(limits Limits) (*conditionsPool, error) {
	env, err := apolcompiler.NewConditionsEnv()
	if err != nil {
		return nil, err
	}
	return &conditionsPool{
		env:      env,
		sem:      make(chan struct{}, limits.MaxConcurrent),
		programs: lru.New(limits.ConditionCacheSize),
	}, nil
}

// NewEngine returns an Engine backed by the given provider.
func NewEngine(provider Provider, opts ...Option) (Engine, error) {
	e := &engine{provider: provider, limits: DefaultLimits()}
	for _, opt := range opts {
		opt(e)
	}
	var err error
	if e.authorize, err = newConditionsPool(e.limits); err != nil {
		return nil, err
	}
	if e.review, err = newConditionsPool(e.limits); err != nil {
		return nil, err
	}
	return e, nil
}

func (p *conditionsPool) acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case p.sem <- struct{}{}:
		return func() { <-p.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// evaluation is the per-request evaluation state.
type evaluation struct {
	limits  Limits
	request map[string]any
	spent   atomic.Uint64
}

func (ev *evaluation) spend(details *cel.EvalDetails) {
	if details == nil {
		return
	}
	if cost := details.ActualCost(); cost != nil {
		ev.spent.Add(*cost)
	}
}

// stopped returns why evaluation must stop, if it must.
func (ev *evaluation) stopped(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("evaluation interrupted: %w", err)
	}
	if spent := ev.spent.Load(); spent > ev.limits.TotalCostLimit {
		return fmt.Errorf("total CEL cost budget exhausted (%d > %d)", spent, ev.limits.TotalCostLimit)
	}
	return nil
}

func (e *engine) Authorize(ctx context.Context, spec *authorizationv1.SubjectAccessReviewSpec, conditionsAware bool) authorizationv1.ConditionsAwareDecision {
	if spec == nil {
		return deny("failed closed: missing SubjectAccessReview spec", "spec is required")
	}
	release, err := e.authorize.acquire(ctx)
	if err != nil {
		return deny("failed closed: no evaluation slot", err.Error())
	}
	defer release()
	snapshot, err := e.provider.Snapshot(ctx)
	if err == nil && snapshot == nil {
		err = ErrNoSnapshot
	}
	if err != nil {
		// Without a snapshot we cannot know whether a Deny applies; fail closed.
		return deny("failed closed: no authorizing policy snapshot", err.Error())
	}
	var applicable []CompiledPolicy
	for _, p := range snapshot.policies {
		if p.Applies(spec) {
			applicable = append(applicable, p)
		}
	}
	if len(applicable) == 0 {
		return noOpinion("no applicable policy", "")
	}
	res := newRuleResult()
	if err := e.checkRequest(spec); err != nil {
		// The request is too large to evaluate safely: every applicable rule errors.
		for _, p := range applicable {
			addPolicyError(res, p.Policy(), err)
		}
		return decide(spec, conditionsAware, res)
	}
	ev := &evaluation{limits: e.limits, request: RequestValue(spec)}
	for _, p := range applicable {
		e.evaluatePolicy(ctx, ev, p, res)
	}
	if err := ev.stopped(ctx); err != nil {
		// The cost budget or the context ran out part way. Which rules were
		// evaluated before that depends on policy names and rule order, so
		// discard the partial results and treat every applicable rule as an
		// error: the outcome is Deny if any applicable Deny or NoOpinion rule
		// exists, and otherwise NoOpinion, regardless of order.
		res = newRuleResult()
		for _, p := range applicable {
			addPolicyError(res, p.Policy(), err)
		}
	}
	d := decide(spec, conditionsAware, res)
	logger.V(4).Info("authorized", "snapshot", snapshot.Generation(), "policies", len(applicable), "decision", d.Type, "cost", ev.spent.Load())
	return d
}

func (e *engine) checkRequest(spec *authorizationv1.SubjectAccessReviewSpec) error {
	raw, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("cannot encode request: %w", err)
	}
	if len(raw) > e.limits.MaxRequestBytes {
		return fmt.Errorf("request is %d bytes, more than the %d byte limit", len(raw), e.limits.MaxRequestBytes)
	}
	return nil
}

func addPolicyError(res *ruleResult, pol *policiesv1alpha1.AuthorizingPolicy, err error) {
	for _, rule := range pol.Spec.Rules {
		res.addError(rule.Effect, conditionID(pol.Name, rule.Name), err)
	}
}

func (e *engine) evaluatePolicy(ctx context.Context, ev *evaluation, p CompiledPolicy, res *ruleResult) {
	pol := p.Policy()
	if cerr := p.CompileError(); cerr != nil {
		addPolicyError(res, pol, fmt.Errorf("policy failed to compile: %w", cerr))
		return
	}
	cp, ok := p.(*compiledPolicy)
	if !ok || cp.compiled == nil {
		addPolicyError(res, pol, errors.New("policy is not compiled"))
		return
	}
	compiled := cp.compiled
	lazyVars := lazy.NewMapValue(celcompiler.VariablesType)
	activation, err := cel.PartialVars(map[string]any{
		celcompiler.RequestKey:   ev.request,
		celcompiler.VariablesKey: lazyVars,
	},
		cel.AttributePattern(celcompiler.ObjectKey),
		cel.AttributePattern(celcompiler.OldObjectKey),
	)
	if err != nil {
		addPolicyError(res, pol, err)
		return
	}
	for _, v := range compiled.Variables {
		lazyVars.Append(v.Name, func(*lazy.MapValue) ref.Val {
			if err := ev.stopped(ctx); err != nil {
				return types.WrapErr(err)
			}
			out, details, err := v.Program.ContextEval(ctx, activation)
			ev.spend(details)
			if err != nil {
				return types.WrapErr(err)
			}
			return out
		})
	}
	for _, rule := range compiled.Rules {
		id := conditionID(pol.Name, rule.Name)
		if err := ev.stopped(ctx); err != nil {
			res.addError(rule.Effect, id, err)
			continue
		}
		out, details, err := rule.Program.ContextEval(ctx, activation)
		ev.spend(details)
		switch {
		case err != nil:
			res.addError(rule.Effect, id, err)
		case types.IsUnknown(out):
			expr, err := e.residual(ev, compiled, rule, details, lazyVars)
			if err != nil {
				res.addError(rule.Effect, id, err)
				continue
			}
			res.addResidual(rule.Effect, authorizationv1.Condition{
				ID:          id,
				Condition:   expr,
				Type:        ConditionType,
				Description: truncate(rule.Description, authorizer.MaxConditionDescriptionBytes-3),
			})
		case out == types.True:
			res.addDefinite(rule.Effect, id)
		case out == types.False:
		default:
			res.addError(rule.Effect, id, fmt.Errorf("expression returned non-boolean %v", out))
		}
	}
}
