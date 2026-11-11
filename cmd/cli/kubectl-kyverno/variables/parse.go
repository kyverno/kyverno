package variables

import (
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
)

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
