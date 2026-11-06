package schema

import "strings"

// Exposure limits which parts of a validated payload CEL can see. Patterns are
// JSON Pointers: "/a/b" matches that node and its descendants, "/a/**" the same,
// and "/**" everything. Exclude wins over include.
type Exposure struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

func (e *Exposure) apply(v any) any {
	if e == nil {
		return v
	}
	inc := e.Include
	if len(inc) == 0 {
		inc = []string{"/**"}
	}
	out, _ := walkExposure(v, "", inc, e.Exclude)
	return out
}

func walkExposure(v any, path string, inc, exc []string) (any, bool) {
	if matchesAny(exc, path) {
		return nil, false
	}
	included := matchesAny(inc, path)
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, c := range t {
			if cv, keep := walkExposure(c, path+"/"+escapeToken(k), inc, exc); keep {
				out[k] = cv
			}
		}
		return out, included || len(out) > 0
	case []any:
		out := make([]any, 0, len(t))
		for i, c := range t {
			if cv, keep := walkExposure(c, path+"/"+itoa(i), inc, exc); keep {
				out = append(out, cv)
			}
		}
		return out, included || len(out) > 0
	default:
		return v, included
	}
}

func matchesAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if matchPattern(p, path) {
			return true
		}
	}
	return false
}

func matchPattern(pat, path string) bool {
	if pat == "/**" {
		return true
	}
	prefix := strings.TrimSuffix(pat, "/**")
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func escapeToken(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{digits[i%10]}, b...)
	}
	return string(b)
}
