package variables

import (
	"strings"
)

func NeedsVariables(vars ...string) bool {
	for _, v := range vars {
		if !strings.Contains(v, "request.object") && !strings.Contains(v, "element") && v != "elementIndex" {
			return true
		}
	}
	return false
}
