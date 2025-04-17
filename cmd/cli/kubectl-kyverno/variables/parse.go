package variables

import (
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
)

func parse(vars ...string) map[string]string {
	result := map[string]string{}
	for _, variable := range vars {
		variable = strings.TrimSpace(variable)
		kvs := strings.Split(variable, "=")
		if len(kvs) != 2 {
			log.Log.Info("ignored variable", "variable", variable)
			continue
		}
		key := strings.TrimSpace(kvs[0])
		value := strings.TrimSpace(kvs[1])
		if len(value) == 0 || len(key) == 0 {
			log.Log.Info("ignored variable", "variable", variable)
			continue
		}
		if strings.Contains(key, "request.object.") {
			log.Log.Info("ignored variable (contains `request.object.`)", "variable", variable)
			continue
		}
		if result[key] != "" {
			log.Log.Info("ignored variable (duplicated)", "variable", variable)
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
