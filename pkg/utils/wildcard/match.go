// Package wildcard matches strings against patterns made of the '*' and '?' wildcards.
package wildcard

import (
	"strings"
	"unicode/utf8"
)

// Match reports whether name matches the wildcard pattern.
//
// The pattern syntax is:
//
//	'*' matches any sequence of characters, including the empty one
//	'?' matches any single character
//	any other character matches itself
//
// There is no escaping and there are no character classes. Unlike path.Match, the
// wildcards also match path separators and a pattern can't be malformed.
//
// Matching is done rune by rune. Invalid UTF-8 bytes are treated as U+FFFD, the same
// way a range loop over the string would.
//
// Match doesn't allocate and runs in O(len(pattern) * len(name)) time in the worst
// case, patterns such as "a*a*a*a*b" can't cause exponential backtracking.
func Match(pattern, name string) bool {
	// px and nx are the offsets of the next rune to match in pattern and name.
	// When matching fails, the last '*' seen is grown by one rune and matching resumes
	// right after it: starPx is the offset in pattern of what follows that '*' and
	// starNx the offset in name of what it doesn't cover yet.
	// Earlier '*' are never reconsidered. What precedes the last '*' has matched the
	// shortest possible prefix of name, and anything an earlier '*' could match more
	// of, the last one can match too. This is what bounds the running time, see
	// https://research.swtch.com/glob for the details.
	px, nx := 0, 0
	starPx, starNx := -1, 0
	for nx < len(name) {
		if px < len(pattern) {
			if pattern[px] == '*' {
				px++
				if px == len(pattern) {
					// a trailing '*' matches whatever is left of name
					return true
				}
				starPx, starNx = px, nx
				continue
			}
			p, pw := utf8.DecodeRuneInString(pattern[px:])
			n, nw := utf8.DecodeRuneInString(name[nx:])
			if p == '?' || p == n {
				px += pw
				nx += nw
				continue
			}
		}
		if starPx < 0 {
			return false
		}
		_, w := utf8.DecodeRuneInString(name[starNx:])
		starNx += w
		px, nx = starPx, starNx
	}
	// name is exhausted, what is left of the pattern must match the empty string
	return strings.TrimLeft(pattern[px:], "*") == ""
}
