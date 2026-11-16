package wildcard

import (
	"strings"
	"testing"
)

// referenceMatch is a test-only reproduction of the previous (exponential)
// matching behavior this package used to delegate to. It exists purely to
// differentially validate Match's iterative rewrite: it reproduces the
// wrapper guards (empty pattern, bare "*"), the []rune conversion, and the
// doubly-recursive backtracking core, in that order — the empty-pattern and
// bare-"*" sweep cases would false-mismatch against a core-only reference.
//
// The core is exponential in the worst case, so the search-space bounds on
// the sweep tests below are load-bearing for suite runtime: widening them
// "for thoroughness" will hang the suite.
func referenceMatch(pattern, name string) bool {
	if pattern == "" {
		return name == pattern
	}
	if pattern == "*" {
		return true
	}
	return referenceDeepMatchRune([]rune(name), []rune(pattern))
}

func referenceDeepMatchRune(str, pattern []rune) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		default:
			if len(str) == 0 || str[0] != pattern[0] {
				return false
			}
		case '?':
			if len(str) == 0 {
				return false
			}
		case '*':
			return referenceDeepMatchRune(str, pattern[1:]) ||
				(len(str) > 0 && referenceDeepMatchRune(str[1:], pattern))
		}
		str = str[1:]
		pattern = pattern[1:]
	}
	return len(str) == 0 && len(pattern) == 0
}

// TestMatchEqualsReferenceASCII exhaustively compares Match against
// referenceMatch over every pattern of length 0..4 and every name of length
// 0..4, both built from {a, b, *, ?}. The name alphabet deliberately
// includes '*' and '?' themselves -- a literal wildcard character in the
// name is exactly the shape that catches a matcher which mishandles a
// pattern '*' colliding with a name rune that is also '*' or '?' (treating
// it as a one-rune literal match instead of the wildcard operator). 341
// patterns x 341 names, single-threaded, trivial under -race.
func TestMatchEqualsReferenceASCII(t *testing.T) {
	patterns := generateStrings([]rune{'a', 'b', '*', '?'}, 4)
	names := generateStrings([]rune{'a', 'b', '*', '?'}, 4)

	for _, p := range patterns {
		for _, n := range names {
			if Match(p, n) != referenceMatch(p, n) {
				t.Fatalf("Match(%q, %q) = %v, referenceMatch(%q, %q) = %v",
					p, n, Match(p, n), p, n, referenceMatch(p, n))
			}
		}
	}
}

// TestMatchEqualsReferenceMultibyte re-runs the differential sweep over a
// multi-byte alphabet, again with '*' and '?' present in the name alphabet.
// This is the sweep that catches a byte-wise rewrite; the ASCII sweep
// cannot, because over {a,b} every rune is one byte. Kept short (length
// 0..3) so the space stays trivial.
func TestMatchEqualsReferenceMultibyte(t *testing.T) {
	patterns := generateStrings([]rune{'a', 'é', '*', '?'}, 3)
	names := generateStrings([]rune{'a', 'é', '*', '?'}, 3)

	for _, p := range patterns {
		for _, n := range names {
			if Match(p, n) != referenceMatch(p, n) {
				t.Fatalf("Match(%q, %q) = %v, referenceMatch(%q, %q) = %v",
					p, n, Match(p, n), p, n, referenceMatch(p, n))
			}
		}
	}
}

// TestMatchTerminatesOnPathologicalInput feeds the input shape that was
// exponential in the previous recursive implementation -- many "a*" segments
// against a long run of "a", which share a matching prefix and so force the
// old matcher to fork at every segment instead of failing fast on the first
// rune. It uses 24 segments over 60 runes, the scale at which the old matcher
// took seconds; the former 12/30 shape completed in well under a second even
// unfixed, so it could not tell fixed from vulnerable.
//
// It asserts the result is correct AND that the work stays within the O(n*m)
// envelope, using the deterministic step counter rather than a wall clock
// (time assertions flake). Any future change reintroducing superlinear
// backtracking trips the step bound. A test in this already-fixed tree cannot
// fail on a revert that deletes matchWithSteps; what it buys is a standing
// guard against superlinear work being reintroduced.
func TestMatchTerminatesOnPathologicalInput(t *testing.T) {
	pattern := strings.Repeat("a*", 24) + "b"
	name := strings.Repeat("a", 60)

	matched, steps := matchWithSteps(pattern, name)
	if matched {
		t.Fatalf("Match(%q, %q) = true, want false", pattern, name)
	}
	if envelope := len(pattern) * len(name); steps > envelope {
		t.Fatalf("matchWithSteps(%q, %q) used %d steps, exceeds O(n*m) envelope %d -- superlinear work reintroduced", pattern, name, steps, envelope)
	}
}

// generateStrings returns every string of length 0..maxLen (inclusive)
// built from alphabet, in increasing-length order.
func generateStrings(alphabet []rune, maxLen int) []string {
	result := []string{""}
	current := []string{""}
	for length := 1; length <= maxLen; length++ {
		next := make([]string, 0, len(current)*len(alphabet))
		for _, s := range current {
			for _, r := range alphabet {
				next = append(next, s+string(r))
			}
		}
		result = append(result, next...)
		current = next
	}
	return result
}
