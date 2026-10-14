package operator

import (
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/ext/wildcard"
)

// element is a value coerced for comparison by the In-family operators
// (In, NotIn, AnyIn, AnyNotIn, AllIn, AllNotIn). It is the single place
// that decides what may be compared and how it stringifies, shared by
// every exact-equality and wildcard comparison site in this package.
//
// Scalars (string and the numeric/bool types already accepted elsewhere in
// this package) stringify into str and are comparable. Everything else --
// map, slice, nil -- is opaque: it never equals anything under exact
// equality, but it can still satisfy a universal "*" pattern under
// wildcard matching (see wildcardElements), so a values: ["*"] catch-all
// over a composite array keeps working.
type element struct {
	str        string
	comparable bool
}

// toElement coerces a single value into an element.
func toElement(v interface{}) element {
	switch t := v.(type) {
	case string:
		return element{str: t, comparable: true}
	case int, int32, int64, float32, float64, bool:
		return element{str: fmt.Sprint(t), comparable: true}
	default:
		return element{}
	}
}

// toElements coerces every entry of a slice into an element.
func toElements(values []interface{}) []element {
	elements := make([]element, 0, len(values))
	for _, v := range values {
		elements = append(elements, toElement(v))
	}
	return elements
}

// elementsFromStrings coerces a []string, such as one decoded from a JSON
// array of strings, into elements.
func elementsFromStrings(values []string) []element {
	elements := make([]element, 0, len(values))
	for _, v := range values {
		elements = append(elements, toElement(v))
	}
	return elements
}

// isUniversalPattern reports whether a pattern consists solely of one or
// more '*' runes -- the shape that matches any string, including one that
// an opaque element has no literal form to compare against.
func isUniversalPattern(p string) bool {
	return p != "" && strings.Trim(p, "*") == ""
}

// equalElements is the exact-equality pair predicate. Only two comparable
// elements with equal coerced strings are equal; an opaque element never
// equals anything, including another opaque element.
func equalElements(a, b element) bool {
	return a.comparable && b.comparable && a.str == b.str
}

// wildcardElements is the symmetric-wildcard pair predicate. Two
// comparable elements match if either's coerced string is a wildcard
// pattern the other satisfies. An opaque element matches only a universal
// "*"/"**" pattern on the comparable side; two opaque elements never
// match.
//
// This is for the array-set paths (the []interface{} value branch and the
// Any/All set helpers), which already matched symmetrically at base.
func wildcardElements(a, b element) bool {
	switch {
	case a.comparable && b.comparable:
		return wildcard.Match(a.str, b.str) || wildcard.Match(b.str, a.str)
	case a.comparable:
		return isUniversalPattern(a.str)
	case b.comparable:
		return isUniversalPattern(b.str)
	default:
		return false
	}
}

// wildcardElementDirectional is the directional wildcard predicate for the
// scalar-value paths (a bare string or bare scalar "value" operand): only
// pattern -> key matching is checked, never the reverse. At base, the
// scalar-value branches of keyExistsInArray/anyKeyExistsInArray/
// allKeyExistsInArray did wildcard.Match(value, key) -- value is the glob
// pattern, key is the concrete string being tested against it. Using the
// symmetric predicate here would let a resource-controlled key itself act
// as a pattern (e.g. a key of "*" would then "match" any policy-declared
// value), which base never allowed.
func wildcardElementDirectional(pattern, key element) bool {
	switch {
	case pattern.comparable && key.comparable:
		return wildcard.Match(pattern.str, key.str)
	case pattern.comparable:
		// key is opaque: only a universal pattern can be said to match
		// data with no literal form, preserving the values: ["*"]
		// catch-all carve-out in the correct direction.
		return isUniversalPattern(pattern.str)
	default:
		// an opaque pattern has no literal text to match with
		return false
	}
}
