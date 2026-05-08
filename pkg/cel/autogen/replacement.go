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

// Apply performs replacements for each Replacement rule using replace(),
// which does a single left-to-right scan per rule and never re-scans
// already-emitted output. This prevents double-replacement corruption:
// each replacement is applied exactly once, and text produced by an earlier
// replacement is never re-examined by a later one because the scan position
// always advances past the emitted output.
func Apply(data []byte, replacements ...Replacement) []byte {
	for _, r := range replacements {
		data = replace(data, []byte("object."+r.From), []byte("object."+r.To))
		data = replace(data, []byte("oldObject."+r.From), []byte("oldObject."+r.To))
	}
	return data
}
