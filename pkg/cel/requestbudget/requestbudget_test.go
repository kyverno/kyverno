package requestbudget

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharge_DeductsUntilTheBudgetIsSpent(t *testing.T) {
	ctx, release := Ensure(context.Background())
	defer release()

	require.NoError(t, Charge(ctx, defaultBudget/2))
	require.NoError(t, Charge(ctx, defaultBudget/4))
	require.NoError(t, Exhausted(ctx), "a partly spent budget is not exhausted")

	err := Charge(ctx, defaultBudget/2)
	require.Error(t, err, "a charge larger than what remains must be refused")
	assert.ErrorIs(t, err, ErrBudgetExceeded)
}

func TestCharge_ExactlyTheRemainderExhaustsTheBudget(t *testing.T) {
	// Charge refuses at units >= remaining, not units > remaining: a request that lands exactly
	// on the ceiling has spent it. Pin this so an off-by-one change is a deliberate decision.
	ctx, _ := Ensure(context.Background())

	require.NoError(t, Charge(ctx, defaultBudget-1))
	err := Charge(ctx, 1)
	require.ErrorIs(t, err, ErrBudgetExceeded, "charging exactly the remainder must exhaust the budget")
	assert.ErrorIs(t, Exhausted(ctx), ErrBudgetExceeded)
}

func TestCharge_KeepsReportingTheChargeThatExhaustedTheBudget(t *testing.T) {
	// Once spent, every later refusal names the charge that emptied the counter, not the small
	// trailing charge that happened to be refused. That is what makes the log line actionable.
	ctx, _ := Ensure(context.Background())

	require.ErrorIs(t, Charge(ctx, defaultBudget+12345), ErrBudgetExceeded)

	err := Charge(ctx, 72)
	require.ErrorIs(t, err, ErrBudgetExceeded)
	assert.Contains(t, err.Error(), "already exhausted by an earlier charge of 10012345")

	err = Exhausted(ctx)
	require.ErrorIs(t, err, ErrBudgetExceeded)
	assert.Contains(t, err.Error(), "already exhausted by an earlier charge of 10012345")
}

func TestExhausted_IsNilUntilACallerIsRefused(t *testing.T) {
	ctx, _ := Ensure(context.Background())
	assert.NoError(t, Exhausted(ctx), "a fresh budget is not exhausted")

	require.NoError(t, Charge(ctx, 1))
	assert.NoError(t, Exhausted(ctx))

	require.Error(t, Charge(ctx, defaultBudget))
	assert.ErrorIs(t, Exhausted(ctx), ErrBudgetExceeded)
}

func TestNoBudgetOnContext_DegradesGracefully(t *testing.T) {
	// A missed Ensure must lose the bound for that call chain, not fail every evaluation on it.
	ctx := context.Background()
	assert.NoError(t, Charge(ctx, defaultBudget*100))
	assert.NoError(t, Charge(ctx, defaultBudget*100))
	assert.NoError(t, Exhausted(ctx))
}

func TestEnsure_ReusesTheBudgetAlreadyOnTheContext(t *testing.T) {
	// An engine's Handle calls Ensure again inside a caller that already did. Both must charge the
	// same counter, or each nested call would get a fresh ceiling.
	outer, _ := Ensure(context.Background())
	inner, _ := Ensure(outer)

	require.NoError(t, Charge(outer, defaultBudget-10))
	assert.ErrorIs(t, Charge(inner, 10), ErrBudgetExceeded, "the nested context must share the outer counter")
	assert.ErrorIs(t, Exhausted(outer), ErrBudgetExceeded, "exhausting through the nested context must be visible on the outer one")
}

func TestEnsure_InsideEnsureBackgroundKeepsTheBackgroundCeiling(t *testing.T) {
	// The reports scanner calls EnsureBackground and then an engine's Handle, which calls Ensure.
	// The outer, smaller ceiling must bind; replacing it would give the 128Mi background pod the
	// admission controller's 384Mi-sized budget.
	ctx, _ := EnsureBackground(context.Background())
	ctx, _ = Ensure(ctx)

	require.NoError(t, Charge(ctx, backgroundBudget-1))
	err := Charge(ctx, 1)
	require.ErrorIs(t, err, ErrBudgetExceeded, "Ensure must not raise the ceiling set by EnsureBackground")
	assert.Contains(t, err.Error(), "of 3000000")
}

func TestEnsureBackground_InsideEnsureKeepsTheAdmissionCeiling(t *testing.T) {
	// The reverse nesting: the outermost entry point owns the limit, whichever it is.
	ctx, _ := Ensure(context.Background())
	ctx, _ = EnsureBackground(ctx)

	assert.NoError(t, Charge(ctx, backgroundBudget+1), "EnsureBackground must not lower a ceiling an outer Ensure already set")
}

func TestEnsure_SeparateRequestsGetSeparateBudgets(t *testing.T) {
	// Two admission requests must not drain each other's budget.
	first, _ := Ensure(context.Background())
	second, _ := Ensure(context.Background())

	require.ErrorIs(t, Charge(first, defaultBudget), ErrBudgetExceeded)
	assert.NoError(t, Charge(second, defaultBudget-1), "an exhausted request must not affect another request")
	assert.NoError(t, Exhausted(second))
}

func TestCharge_ConcurrentChargesNeverOverspend(t *testing.T) {
	// Policies in one request can be evaluated concurrently. Under contention the counter must
	// admit exactly as many charges as fit, never more. With charges of 1000 against 10,000,000
	// and refusal at units >= remaining, exactly 9,999 fit, whatever the interleaving.
	ctx, _ := Ensure(context.Background())
	const (
		workers   = 64
		perWorker = 500 // 64*500 = 32,000 attempts, far more than fit
		units     = 1000
	)
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				if err := Charge(ctx, units); err == nil {
					admitted.Add(1)
				} else if !errors.Is(err, ErrBudgetExceeded) {
					t.Errorf("unexpected error: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(defaultBudget/units-1), admitted.Load())
	assert.ErrorIs(t, Exhausted(ctx), ErrBudgetExceeded)
}

func TestErrBudgetExceeded_MessagesNameTheLimit(t *testing.T) {
	ctx, _ := EnsureBackground(context.Background())
	err := Charge(ctx, backgroundBudget*2)
	require.ErrorIs(t, err, ErrBudgetExceeded)
	assert.True(t, strings.HasPrefix(err.Error(), ErrBudgetExceeded.Error()), "the sentinel must lead the message: %q", err)
	assert.Contains(t, err.Error(), "charged 6000000, 3000000 remaining (of 3000000)")
}
