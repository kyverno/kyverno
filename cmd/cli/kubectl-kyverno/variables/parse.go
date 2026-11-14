package variables

import (
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
)

// parse turns "key=value" entries (as given to --set) into a map.
//
// Each entry is split on its first "=" so values may themselves contain "="
// (URLs with query strings, base64 padding, connection strings). Entries are
// ignored, with a V(3) log line, when they have no "=", an empty key or value,
// a key containing "request.object.", or a key already seen (first one wins).
// It returns nil when no entry was kept.
func parse(vars ...string) map[string]string {
	result := map[string]string{}
	for _, variable := range vars {
		variable = strings.TrimSpace(variable)
		// split on the first "=" only so that values containing "=" (URLs with
		// query strings, base64 padding, connection strings, ...) are preserved
		rawKey, rawValue, found := strings.Cut(variable, "=")
		if !found {
			log.Log.V(3).Info("ignored variable", "variable", variable)
			continue
		}
		key := strings.TrimSpace(rawKey)
		value := strings.TrimSpace(rawValue)
		if len(value) == 0 || len(key) == 0 {
			log.Log.V(3).Info("ignored variable", "variable", variable)
			continue
		}
		if strings.Contains(key, "request.object.") {
			log.Log.V(3).Info("ignored variable (contains `request.object.`)", "variable", variable)
			continue
		}
		if result[key] != "" {
			log.Log.V(3).Info("ignored variable (duplicated)", "variable", variable)
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
