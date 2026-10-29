package jmespath

import (
	"regexp"
	"sort"

	"github.com/kyverno/kyverno/pkg/config"
)

// callPattern matches a bare identifier immediately followed by "(", i.e. a function call
// site in a JMESPath expression (JMESPath has no other syntax that produces this shape).
var callPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*\(`)

// knownFunctionNames is the fixed set of custom JMESPath function names Kyverno registers
// (see GetFunctions); it doesn't depend on configuration, so a default configuration is enough
// to compute it once.
var knownFunctionNames = func() map[string]struct{} {
	names := make(map[string]struct{})
	for _, f := range GetFunctions(config.NewDefaultConfiguration(false)) {
		names[f.Name] = struct{}{}
	}
	return names
}()

// UsedFunctionNames returns, sorted, which of Kyverno's known custom JMESPath functions appear
// as call sites in expr. It never returns names outside that fixed registry, so callers can use
// the result as a metric label without risking unbounded cardinality from the expression text
// itself.
func UsedFunctionNames(expr string) []string {
	matches := callPattern.FindAllString(expr, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		name := m[:len(m)-1] // strip trailing "("
		if _, ok := knownFunctionNames[name]; !ok {
			continue
		}
		seen[name] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
