package autogen

import (
	"bytes"
)

// protectedMetadataFields lists ObjectMeta fields that identify the workload
// resource itself. These must stay anchored to the controller object during
// autogen and must never be rewritten into a pod template path. Pod-relevant
// metadata (labels, annotations) is intentionally omitted so it continues to
// rewrite to the pod template.
var protectedMetadataFields = []string{
	"namespace",
	"name",
	"generateName",
	"uid",
	"resourceVersion",
	"generation",
	"creationTimestamp",
	"deletionTimestamp",
	"deletionGracePeriodSeconds",
	"finalizers",
	"ownerReferences",
	"managedFields",
}

func buildProtectedMetadataSuffixes(fields []string) (dotSuffixes, bracketSuffixes [][]byte) {
	for _, field := range fields {
		dotSuffixes = append(dotSuffixes, []byte("."+field))
		bracketSuffixes = append(bracketSuffixes,
			[]byte(`["`+field+`"]`),
			[]byte(`['`+field+`']`),
		)
	}
	return dotSuffixes, bracketSuffixes
}

var protectedSuffixes, protectedBracketSuffixes = buildProtectedMetadataSuffixes(protectedMetadataFields)

type Replacement struct {
	From string
	To   string
}

func (r *Replacement) Apply(data []byte) []byte {
	data = replace(data, []byte("object."+r.From), []byte("object."+r.To))
	data = replace(data, []byte("oldObject."+r.From), []byte("oldObject."+r.To))
	return data
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
// begins with any of the protected suffixes as a complete path segment.
func isProtected(rest []byte) bool {
	for _, suffix := range protectedSuffixes {
		if isCompleteSuffix(rest, suffix) {
			return true
		}
	}
	for _, suffix := range protectedBracketSuffixes {
		if bytes.HasPrefix(rest, suffix) {
			return true
		}
	}
	return false
}

// isCompleteSuffix reports whether rest begins with suffix as a full path
// segment. The suffix must either end the expression or be followed by a
// non-identifier character so that fields like `metadata.name` are protected
// while hypothetical fields like `metadata.nameFoo` are not.
func isCompleteSuffix(rest, suffix []byte) bool {
	if !bytes.HasPrefix(rest, suffix) {
		return false
	}
	next := rest[len(suffix):]
	return len(next) == 0 || !isIdentifierByte(next[0])
}

// isIdentifierByte reports whether b can be part of a CEL identifier segment.
func isIdentifierByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func Apply(data []byte, replacements ...Replacement) []byte {
	for _, replacement := range replacements {
		data = replacement.Apply(data)
	}
	return data
}
