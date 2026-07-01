package autogen

import (
	"bytes"
)

// protectedSuffixes lists field paths that must remain anchored to the
// workload object's own metadata and must never be rewritten into a pod
// template path. For example, `object.metadata.namespace` must stay as-is
// because pod templates (e.g. on Deployments) usually do not carry a
// `metadata.namespace` field, which would otherwise break match conditions.
var protectedSuffixes = [][]byte{
	[]byte(".namespace"),
}

type Replacement struct {
	From string
	To   string
}

func (r *Replacement) Apply(data []byte) []byte {
	return Apply(data, *r)
}

// replace rewrites every occurrence of from with to, except occurrences that
// are immediately followed by a protected suffix (e.g. `.namespace`). Unlike a
// sentinel/placeholder swap, this never injects synthetic markers into the
// data, so it cannot collide with or corrupt user-provided content such as CEL
// expressions.
func replace(data, from, to []byte) []byte {
	if len(from) == 0 || bytes.Equal(from, to) {
		return data
	}
	// Fast path: if from never occurs, return data unchanged without
	// allocating or copying for the common no-op case.
	idx := bytes.Index(data, from)
	if idx < 0 {
		return data
	}
	// Pre-size the buffer. When the replacement expands the input (the common
	// case, e.g. `object.spec` -> `object.spec.template.spec`), grow by an
	// upper bound on the final size so the buffer never has to reallocate
	// mid-loop. Protected occurrences are left unchanged, so the real size is
	// at most this estimate.
	size := len(data)
	if len(to) > len(from) {
		size += bytes.Count(data, from) * (len(to) - len(from))
	}
	var buf bytes.Buffer
	buf.Grow(size)
	for idx >= 0 {
		buf.Write(data[:idx])
		rest := data[idx+len(from):]
		if isProtected(rest) {
			// Leave this occurrence untouched and continue scanning after it.
			buf.Write(from)
		} else {
			buf.Write(to)
		}
		data = rest
		idx = bytes.Index(data, from)
	}
	buf.Write(data)
	return buf.Bytes()
}

// isProtected reports whether rest (the bytes immediately following a match)
// begins with any of the protected suffixes as a complete path segment. The
// suffix must either end the expression or be followed by a non-identifier
// character so that fields like `metadata.namespace` are protected while
// hypothetical fields like `metadata.namespaceFoo` are not.
func isProtected(rest []byte) bool {
	for _, suffix := range protectedSuffixes {
		if !bytes.HasPrefix(rest, suffix) {
			continue
		}
		next := rest[len(suffix):]
		if len(next) == 0 || !isIdentifierByte(next[0]) {
			return true
		}
	}
	return false
}

// isIdentifierByte reports whether b can be part of a CEL identifier segment.
func isIdentifierByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

// rule pairs a from pattern with its to replacement.
type rule struct {
	from []byte
	to   []byte
}

// Apply performs all substitutions in a single left-to-right scan over data,
// processing all Replacement rules simultaneously. At each position it checks
// every rule's pattern and applies the first (longest) match, then advances
// past the replaced text without re-scanning it. This guarantees that output
// produced by one substitution is never examined again by any other rule,
// preventing double-replacement regardless of the order rules are supplied.
// Protected suffixes (e.g. `.namespace`) are honoured: a match whose
// immediately-following bytes form a protected suffix is left unchanged.
func Apply(data []byte, replacements ...Replacement) []byte {
	if len(replacements) == 0 {
		return data
	}

	// Build a flat list of (from, to) byte-slice pairs — two per Replacement
	// (object.X and oldObject.X).
	rules := make([]rule, 0, len(replacements)*2)
	for _, r := range replacements {
		rules = append(rules,
			rule{[]byte("object." + r.From), []byte("object." + r.To)},
			rule{[]byte("oldObject." + r.From), []byte("oldObject." + r.To)},
		)
	}

	var buf bytes.Buffer
	buf.Grow(len(data))

	for len(data) > 0 {
		// Find the earliest match among all rules. Ties are broken by rule
		// order (first rule wins), which is consistent with strings.NewReplacer.
		bestIdx := -1
		bestRule := -1
		for i, r := range rules {
			if len(r.from) == 0 {
				continue
			}
			idx := bytes.Index(data, r.from)
			if idx < 0 {
				continue
			}
			if bestIdx < 0 || idx < bestIdx || (idx == bestIdx && i < bestRule) {
				bestIdx = idx
				bestRule = i
			}
		}

		if bestIdx < 0 {
			// No more matches — flush the rest.
			buf.Write(data)
			break
		}

		// Emit everything before the match unchanged.
		buf.Write(data[:bestIdx])

		r := rules[bestRule]
		rest := data[bestIdx+len(r.from):]
		if isProtected(rest) {
			// Protected: emit the original pattern and advance past it.
			buf.Write(r.from)
		} else {
			// Apply the substitution and advance past the matched pattern.
			buf.Write(r.to)
		}
		// Advance past the match. We do NOT re-scan the emitted output,
		// so no rule can corrupt text that was just written.
		data = rest
	}

	return buf.Bytes()
}
