package requestbudget_test

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
)

// defaultBudget mirrors the package's unexported default ceiling (celconfig.RuntimeCELCostBudget).
const defaultBudget = 10_000_000

// A charge that exactly equals the remaining budget fits and must be allowed. Kubernetes
// refuses its own admission-plugin CEL evaluation only when cost > remainingBudget (see
// k8s.io/apiserver .../plugin/cel/activation.go), so a cost equal to the ceiling passes and
// spends the budget to zero. The package documents that it mirrors that ceiling.
func TestCharge_ExactRemainingIsAllowed(t *testing.T) {
	ctx, release := requestbudget.Ensure(context.Background())
	defer release()

	// Spend all but one unit.
	if err := requestbudget.Charge(ctx, defaultBudget-1); err != nil {
		t.Fatalf("charging under the budget must succeed, got: %v", err)
	}

	// The remaining budget is exactly one unit. A charge of one unit fits exactly and
	// must be allowed.
	if err := requestbudget.Charge(ctx, 1); err != nil {
		t.Fatalf("a charge equal to the remaining budget must be allowed, got: %v", err)
	}

	// The budget is now exactly spent, so any further positive charge must be refused.
	if err := requestbudget.Charge(ctx, 1); err == nil {
		t.Fatal("a charge beyond the spent budget must be refused")
	}
}

// A single charge equal to the full budget must be allowed, not refused at the boundary.
func TestCharge_FullBudgetInOneChargeIsAllowed(t *testing.T) {
	ctx, release := requestbudget.Ensure(context.Background())
	defer release()

	if err := requestbudget.Charge(ctx, defaultBudget); err != nil {
		t.Fatalf("a single charge equal to the whole budget must be allowed, got: %v", err)
	}
}
