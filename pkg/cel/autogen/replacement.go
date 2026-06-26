package autogen

import (
	"strings"
)

type Replacement struct {
	From string
	To   string
}

func (r *Replacement) Apply(data []byte) []byte {
	return Apply(data, *r)
}

// Apply performs all replacements in a single pass using strings.NewReplacer,
// which scans left-to-right and never revisits already-replaced text.
// This prevents double-replacement when a replacement's output contains
// a pattern matched by another replacement (e.g. "metadata" -> "spec.template.metadata"
// contains "spec", which would be re-matched by the "spec" replacement).
func Apply(data []byte, replacements ...Replacement) []byte {
	oldnew := make([]string, 0, len(replacements)*4)
	for _, r := range replacements {
		oldnew = append(oldnew,
			"object."+r.From, "object."+r.To,
			"oldObject."+r.From, "oldObject."+r.To,
		)
	}
	return []byte(strings.NewReplacer(oldnew...).Replace(string(data)))
}
