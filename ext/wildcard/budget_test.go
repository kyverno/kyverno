package wildcard

import (
	"strings"
	"testing"
)

// rewindPattern builds a "*" + "a" x segLen + "b" glob, the shape that
// forces a rewind at every position of a value that has no trailing "b" --
// the binding worst case for the step budget (quadratic only in the value
// side, per the design's work model).
func rewindPattern(segLen int) string {
	return "*" + strings.Repeat("a", segLen) + "b"
}

// TestMatchLargestLegitimateShapeStaysUnderHalfBudget pins the counting
// convention: the binding legitimate case -- a 255-rune annotation glob in
// its worst rewind shape against a 256KiB annotation value -- must not
// trip the budget, and must stay at or under half of it. If a future edit
// changes what counts as a step, this margin (not just "does it trip")
// catches the drift before it silently erodes the safety margin.
func TestMatchLargestLegitimateShapeStaysUnderHalfBudget(t *testing.T) {
	pattern := rewindPattern(253) // "*" + 253 "a"s + "b" == 255 runes
	value := strings.Repeat("a", 256*1024)

	matched, steps := matchWithSteps(pattern, value)

	if matched {
		t.Fatalf("expected no match (value has no trailing b), got true")
	}
	if steps > maxMatchSteps {
		t.Fatalf("expected the largest legitimate case to stay under budget, got %d steps (budget %d)", steps, maxMatchSteps)
	}
	if steps > maxMatchSteps/2 {
		t.Fatalf("expected the largest legitimate case to stay at or under half the budget (counting-convention margin), got %d steps (budget/2 %d)", steps, maxMatchSteps/2)
	}
}

// TestMatchAnswersOverBudgetPairsAsNonMatching asserts the fail-closed
// direction: a pair whose work would exceed the budget is answered as not
// matching, and the reported step count is capped at the trip point
// (budget + 1), not allowed to run further.
func TestMatchAnswersOverBudgetPairsAsNonMatching(t *testing.T) {
	pattern := rewindPattern(256000)
	value := strings.Repeat("a", 500*1024)

	matched, steps := matchWithSteps(pattern, value)

	if matched {
		t.Fatalf("expected a budget trip to answer false, got true")
	}
	if steps != maxMatchSteps+1 {
		t.Fatalf("expected the step count to be capped at the trip point (%d), got %d", maxMatchSteps+1, steps)
	}
}

// TestMatchBudgetTripIsDeterministic asserts that tripping the budget is
// not flaky: the same over-budget pair produces the same (matched, steps)
// result every time.
func TestMatchBudgetTripIsDeterministic(t *testing.T) {
	pattern := rewindPattern(256000)
	value := strings.Repeat("a", 500*1024)

	matched1, steps1 := matchWithSteps(pattern, value)
	matched2, steps2 := matchWithSteps(pattern, value)

	if matched1 != matched2 || steps1 != steps2 {
		t.Fatalf("expected a deterministic result, got (%v, %d) then (%v, %d)", matched1, steps1, matched2, steps2)
	}
}

// TestMatchBudgetDoesNotAffectCheapLargeOperands confirms the budget is
// invisible to legitimately cheap large-operand shapes: a bare star, a
// short prefix glob, and an image-reference-shaped glob all answer
// correctly against large values while using a small fraction of the
// budget.
func TestMatchBudgetDoesNotAffectCheapLargeOperands(t *testing.T) {
	bigValue := strings.Repeat("a", 1024*1024)

	tests := []struct {
		name    string
		pattern string
		value   string
		matched bool
	}{
		{"bare star vs 1MiB value", "*", bigValue, true},
		{"short prefix glob vs 1MiB value", "*foo", bigValue, false},
		{"image-ref-shaped glob vs 1MiB value", "ghcr.io/kyverno/*", bigValue, false},
		{"long pattern vs short value", strings.Repeat("a", 2048) + "*", "short", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, steps := matchWithSteps(tt.pattern, tt.value)
			if matched != tt.matched {
				t.Fatalf("expected matched=%v, got %v", tt.matched, matched)
			}
			if steps > maxMatchSteps/10 {
				t.Fatalf("expected a cheap large-operand shape to use a small fraction of the budget, used %d steps (budget %d)", steps, maxMatchSteps)
			}
		})
	}
}

// TestMatchCountsTrailingStarsAgainstBudget pins that the trailing-'*'
// consumption loop (which runs after the main loop exits, to accept a
// pattern like "a***" against a value of "a") is counted against the step
// budget too. Before this was fixed, this loop's iterations were free,
// leaving one uncounted (if bounded) path through the matcher.
func TestMatchCountsTrailingStarsAgainstBudget(t *testing.T) {
	pattern := "a" + strings.Repeat("*", 1000)
	name := "a"

	matched, steps := matchWithSteps(pattern, name)

	if !matched {
		t.Fatalf("expected a match (trailing stars match zero runes), got false")
	}
	// one step for the main loop consuming "a", plus one step per trailing
	// '*' consumed afterwards.
	want := 1 + 1000
	if steps != want {
		t.Fatalf("expected steps=%d (trailing stars counted), got %d", want, steps)
	}
}
