package variables

import (
	"strings"
)

func NeedsVariable(variable string) bool {
	return variable != "" &&
		!strings.Contains(variable, "request.object") &&
		!strings.Contains(variable, "request.operation") &&
		!strings.Contains(variable, "element") &&
		variable != "elementIndex"
}

func NeedsVariables(variables ...string) bool {
	for _, variable := range variables {
		if NeedsVariable(variable) {
			return true
		}
	}
	return false
}
