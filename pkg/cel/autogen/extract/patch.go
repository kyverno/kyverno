package extract

import (
	"strconv"
	"strings"

	"gomodules.xyz/jsonpatch/v2"
)

// JSONPointerPrefix converts Segments into an RFC-6901 JSON Pointer prefix,
func (e Extracted) JSONPointerPrefix() string {
	var b strings.Builder
	for _, s := range e.Segments {
		b.WriteByte('/')
		if s.IsIdx {
			b.WriteString(strconv.Itoa(s.Index))
		} else {
			b.WriteString(escapeJSONPointer(s.Key))
		}
	}
	return b.String()
}

func escapeJSONPointer(k string) string {
	k = strings.ReplaceAll(k, "~", "~0")
	k = strings.ReplaceAll(k, "/", "~1")
	return k
}

// RebasePatch prefixes every operation's Path in a JSON Patch computed
// against a synthesized Pod, so that it is applied to the real nested location
// inside the parent object instead.
func RebasePatch(ops []jsonpatch.JsonPatchOperation, prefix string) []jsonpatch.JsonPatchOperation {
	out := make([]jsonpatch.JsonPatchOperation, len(ops))
	for i, op := range ops {
		r := op
		r.Path = prefix + op.Path
		out[i] = r
	}
	return out
}
