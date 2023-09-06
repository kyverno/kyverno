package variables

import (
	"strings"
)

func parse(vars ...string) map[string]string {
	result := map[string]string{}
	for _, variable := range vars {
		variable = strings.TrimSpace(variable)
		kvs := strings.Split(variable, "=")
		if len(kvs) != 2 {
			// TODO warning
			continue
		}
		key := strings.TrimSpace(kvs[0])
		value := strings.TrimSpace(kvs[1])
		if len(value) == 0 || len(key) == 0 {
			// TODO log
			continue
		}
		if strings.Contains(key, "request.object.") {
			// TODO log
			continue
		}
		if result[key] != "" {
			// TODO log
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
