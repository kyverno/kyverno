package yaml

import (
	"strings"
)

// IsEmptyDocument checks if a yaml document is empty (contains only comments)
func IsEmptyDocument(document document) bool {
	for _, line := range strings.Split(string(document), "\n") {
		line := strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return false
		}
	}
	return true
}
