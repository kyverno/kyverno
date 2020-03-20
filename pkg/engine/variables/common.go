package variables

import "regexp"

//IsVariable returns true if the element contains a 'valid' variable {{}}
func IsVariable(element string) bool {
	validRegex := regexp.MustCompile(variableRegex)
	groups := validRegex.FindAllStringSubmatch(element, -1)
	return len(groups) != 0
}
