// Package requestbudget is a minimal per-request CEL cost bound: one shared counter across
// every program a single admission request or background reconcile evaluates, so a policy
// with many variables each just under the per-program limit cannot retain an unbounded
// multiple of that limit by spreading the cost across calls instead of one. This is
// deliberately not a port of the full design's pkg/cel/budget: no charge lanes, no
// concurrency slots, no Detach, no exception-namespace keying -- one counter, one ceiling.
package requestbudget

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"k8s.io/klog/v2"
)

// defaultBudget is celconfig.RuntimeCELCostBudget (10,000,000), the same per-request ceiling
// Kubernetes applies to its own admission-plugin CEL evaluation. Hardcoded: no flag, unlike
// the dropped pkg/cel/budget's --celCostBudget. At 0.1 cost units per charged byte
// (StringTraversalCostFactor), this permits roughly 100 MB of charged bytes per request --
// calibrated against the admission controller's chart memory limit (384Mi,
// charts/kyverno/values.yaml).
const defaultBudget = 10_000_000

// backgroundBudget is defaultBudget scaled down for the background and reports controllers,
// whose chart memory limit (128Mi, charts/kyverno/values.yaml) is a third of the admission
// controller's (384Mi): a third of the memory gets a third of the units. Without this, a
// background scan whose policies are each individually under the per-program limit could
// still charge up toward defaultBudget's ~100 MB before the request budget refused it --
// survivable at 384Mi, an OOM kill at 128Mi (measured: 8 costly-but-under-limit expressions
// evaluated against ~900 KB existing resources OOM-kills a 128Mi background/reports
// controller into a crash loop; 1 and 4 survive). If either chart memory default changes,
// recompute this proportion.
const backgroundBudget = 3_000_000

// ErrBudgetExceeded is returned by Charge once the request's counter is spent. Wrapped so
// pkg/cel/compiler's IsResourceError recognizes it the same way it recognizes a per-program
// cost-limit or guard refusal.
var ErrBudgetExceeded = errors.New("cel: per-request cost budget exceeded")

type contextKey struct{}

type budget struct {
	mu        sync.Mutex
	limit     uint64
	remaining uint64
	// exhaustedBy is the charge that first emptied the counter (0 until that happens).
	// Charge keeps reporting this one value on every subsequent call on an already-spent
	// budget, rather than whatever small amount that later call happens to charge --
	// otherwise the error a caller actually sees is often "charged 72, 0 remaining" from a
	// trailing cheap call, not the charge that actually exhausted the request.
	exhaustedBy uint64
}

// Ensure returns a context carrying a fresh per-request budget sized for the admission
// controller (defaultBudget), and a release func kept for symmetry with the caller's usual
// defer-release idiom (there is nothing to release: the counter lives on the context and is
// garbage-collected with it). A ctx that already carries a budget is returned unchanged, so
// a caller that calls an engine's Handle, which calls Ensure again, still shares one counter
// for the whole call chain.
func Ensure(ctx context.Context) (context.Context, func()) {
	return ensure(ctx, defaultBudget)
}

// EnsureBackground is Ensure sized for the background and reports controllers' smaller
// chart memory limit (backgroundBudget's own comment). Use this, not Ensure, at any entry
// point that runs under those controllers' pod, not the admission controller's.
func EnsureBackground(ctx context.Context) (context.Context, func()) {
	return ensure(ctx, backgroundBudget)
}

func ensure(ctx context.Context, limit uint64) (context.Context, func()) {
	if _, ok := ctx.Value(contextKey{}).(*budget); ok {
		return ctx, func() {}
	}
	return context.WithValue(ctx, contextKey{}, &budget{limit: limit, remaining: limit}), func() {}
}

// Charge deducts units from ctx's budget and refuses once it is spent.
//
// Degrades gracefully when ctx carries no budget: charges nothing and never refuses, so a
// missed Ensure/EnsureBackground call at some entry point loses the per-request bound for
// that call chain rather than failing every evaluation on it -- today's behavior, unchanged.
// This is a deliberate trade against the full design's fail-closed ErrNoBudget, which forced
// wiring every CEL evaluation entry point (17 of them) before anything could evaluate at all
// and briefly broke every evaluation path in the process. The advisory this closes is
// rescored Medium (policy authorship is privileged), so a silently unbounded request on a
// missed wiring -- not a silently failed one -- is the accepted trade here. Do not change
// this to fail closed without first auditing every CEL evaluation entry point the way the
// full design's D-series decisions did.
func Charge(ctx context.Context, units uint64) error {
	b, ok := ctx.Value(contextKey{}).(*budget)
	if !ok {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.exhaustedBy > 0 {
		return fmt.Errorf("%w: charged %d of %d, already exhausted by an earlier charge of %d", ErrBudgetExceeded, units, b.limit, b.exhaustedBy)
	}
	if units >= b.remaining {
		remaining := b.remaining
		b.exhaustedBy = units
		b.remaining = 0
		// Refusals otherwise surface only in the admission/report response, not in the
		// controller's own log, which is undiagnosable in production: this is the one
		// line for the whole package, logged exactly once per exhausted budget (guarded
		// by exhaustedBy above), not once per subsequent charge attempt on it.
		klog.V(2).InfoS("cel: per-request cost budget exceeded", "charged", units, "remaining", remaining, "limit", b.limit)
		return fmt.Errorf("%w: charged %d, %d remaining (of %d)", ErrBudgetExceeded, units, remaining, b.limit)
	}
	b.remaining -= units
	return nil
}
