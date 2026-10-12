package compiler

import (
	"math"
	"strings"
	"testing"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// mulSat must saturate rather than wrap: a product used only in a `bound > limit` comparison
// that overflowed int64 to a negative or small value would let the guard pass a bomb.
func TestMulSatSaturates(t *testing.T) {
	if got := mulSat(math.MaxInt64, 2); got != math.MaxInt64 {
		t.Fatalf("overflow must saturate to MaxInt64, got %d", got)
	}
	if got := mulSat(3_000_000, 3_000_000); got != 9_000_000_000_000 {
		t.Fatalf("in-range product must be exact, got %d", got)
	}
	if got := mulSat(0, math.MaxInt64); got != 0 {
		t.Fatalf("zero must be zero, got %d", got)
	}
}

// regexReplaceGuard's backreference branch multiplies three argument-derived lengths. With
// each near a couple of MB the product exceeds int64 and, before saturation, wrapped to a
// value under the cap so the guard passed a replacement that expands to astronomically more
// than the cap. It must refuse.
func TestRegexReplaceGuardRefusesOnOverflowingBound(t *testing.T) {
	const n = 2_200_000 // ~cube-root of MaxInt64: n*n*n overflows int64
	target := strings.Repeat("a", n)
	repl := `\1` + strings.Repeat("b", n-2) // backreference so the three-factor branch runs
	args := []ref.Val{types.String(target), types.String("a"), types.String(repl)}
	if errVal := regexReplaceGuard(args); errVal == nil {
		t.Fatal("expected a guard refusal for an overflowing output bound, got none")
	}
}
