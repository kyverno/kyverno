package autogen

import (
	"bytes"
	"strings"
)

// podTemplateMetadataFields lists the metadata fields that belong to pod
// templates. All other metadata fields remain scoped to the controller being
// admitted because they have no equivalent pod-template meaning.
var podTemplateMetadataFields = []string{"labels", "annotations"}

type Replacement struct {
	From string
	To   string
}

func (r *Replacement) Apply(data []byte) []byte {
	data = r.applyToObject(data, "oldObject")
	data = r.applyToObject(data, "object")
	return data
}

func (r *Replacement) applyToObject(data []byte, object string) []byte {
	if r.From == "metadata" {
		return replacePodTemplateMetadata(data, object, r.To)
	}
	from := []byte(object + "." + r.From)
	to := []byte(object + "." + r.To)
	skipSuffixes := [][]byte{[]byte(r.To[len(r.From):])}
	if strings.HasSuffix(r.To, ".spec") {
		metadataPath := strings.TrimSuffix(r.To, ".spec") + ".metadata"
		skipSuffixes = append(skipSuffixes, []byte(metadataPath[len(r.From):]))
	}
	return replace(data, from, to, skipSuffixes...)
}

// replacePodTemplateMetadata rewrites only metadata fields which are present
// on pod templates. This intentionally leaves bare metadata and every other
// ObjectMeta field anchored to the controller resource.
//
// Replacement happens after JSON marshaling, so double-quoted CEL bracket
// notation also has an escaped representation. This byte-level transformer is
// not CEL-AST-aware and therefore preserves the existing behavior for CEL
// string literals containing matching paths.
func replacePodTemplateMetadata(data []byte, object, metadataReplacement string) []byte {
	for _, field := range podTemplateMetadataFields {
		for _, selector := range []string{".", ".?"} {
			from := []byte(object + ".metadata" + selector + field)
			to := []byte(object + "." + metadataReplacement + selector + field)
			data = replace(data, from, to)
		}

		for _, quote := range []string{`"`, `'`} {
			from := []byte(object + ".metadata[" + quote + field + quote + "]")
			to := []byte(object + "." + metadataReplacement + "[" + quote + field + quote + "]")
			data = replace(data, from, to)
		}

		from := []byte(object + `.metadata[\"` + field + `\"]`)
		to := []byte(object + `.` + metadataReplacement + `[\"` + field + `\"]`)
		data = replace(data, from, to)
	}
	return data
}

// replace rewrites complete CEL path prefixes from with to. It never rewrites
// a longer identifier such as `object.metadata.labelsFoo` and can skip an
// already generated suffix to keep replacements idempotent.
func replace(data, from, to []byte, skipSuffixes ...[]byte) []byte {
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
	// mid-loop. Unchanged occurrences keep the final size at or below this
	// estimate.
	size := len(data)
	if len(to) > len(from) {
		size += bytes.Count(data, from) * (len(to) - len(from))
	}
	var buf bytes.Buffer
	buf.Grow(size)
	for idx >= 0 {
		buf.Write(data[:idx])
		rest := data[idx+len(from):]
		if !isCompletePathPrefix(rest) || hasPrefix(rest, skipSuffixes) {
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

func hasPrefix(data []byte, prefixes [][]byte) bool {
	for _, prefix := range prefixes {
		if bytes.HasPrefix(data, prefix) {
			return true
		}
	}
	return false
}

func isCompletePathPrefix(rest []byte) bool {
	return len(rest) == 0 || !isIdentifierByte(rest[0])
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
