package wildcard

// maxMatchSteps bounds the work a single Match call may perform, counted in
// main-loop iterations (see matchWithSteps). It is sized >= 2x the largest
// legitimate case (a 255-rune annotation glob in its worst rewind shape
// against a 256KiB annotation value ~= 67M steps), not "just fits under" --
// the margin is the contract, so that a shift in counting convention or a
// larger legitimate input cannot silently cross it.
const maxMatchSteps = 1 << 27 // ~134M steps

// Match reports whether name matches the shell-style pattern. The pattern
// supports '*' (matches zero or more runes) and '?' (matches exactly one
// rune, including a path separator). Both operands are compared rune by
// rune, so multi-byte characters count as a single unit for '?' and '*'.
//
// The algorithm below is an iterative, single-backtrack, two-pointer glob
// matcher: each '*' is backtracked to at most once, and every backtrack
// advances the string index it resumes from, so runtime is bounded by
// O(len(name) * len(pattern)) with no recursion.
//
// Match answers within a fixed work budget; pattern/name pairs whose
// evaluation would exceed the budget are reported as not matching.
func Match(pattern, name string) bool {
	matched, _ := matchWithSteps(pattern, name)
	return matched
}

// matchWithSteps is Match's single implementation, additionally reporting
// the number of main-loop iterations it performed. Match wraps this and
// discards the count; the count exists so tests can pin the step-budget
// boundary against the exact same code path Match runs, instead of a
// parallel copy that could drift from it.
func matchWithSteps(pattern, name string) (matched bool, steps int) {
	if pattern == "" {
		return name == "", 0
	}
	if pattern == "*" {
		return true, 0
	}

	p := []rune(pattern)
	s := []rune(name)

	var (
		sIdx, pIdx = 0, 0
		starIdx    = -1 // index in p of the last '*' seen, or -1
		sTmpIdx    = 0  // s index to resume from after that '*'
	)

	for sIdx < len(s) {
		steps++
		if steps > maxMatchSteps {
			return false, steps
		}

		switch {
		case pIdx < len(p) && p[pIdx] == '*':
			// '*' is always a wildcard operator in the pattern, even when the
			// name's current rune is itself a literal '*' or '?' -- it must
			// never be treated as a literal-equality match against them.
			starIdx = pIdx
			sTmpIdx = sIdx
			pIdx++ // '*' provisionally consumes zero runes
		case pIdx < len(p) && (p[pIdx] == '?' || p[pIdx] == s[sIdx]):
			sIdx++
			pIdx++
		case starIdx != -1:
			pIdx = starIdx + 1 // backtrack: let the last '*' consume one more rune
			sTmpIdx++
			sIdx = sTmpIdx
		default:
			return false, steps
		}
	}

	// consume any trailing '*' in the pattern -- counted against the same
	// budget so no path through this function is uncounted, even though
	// this loop is a single linear pass over the pattern and cannot itself
	// be made to blow up.
	for pIdx < len(p) && p[pIdx] == '*' {
		steps++
		if steps > maxMatchSteps {
			return false, steps
		}
		pIdx++
	}
	return pIdx == len(p), steps
}
