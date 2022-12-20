package wildcard

import "strings"

func ContainsWildcard(v string) bool {
	return strings.Contains(v, "*") || strings.Contains(v, "?")
}
