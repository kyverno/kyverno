package yaml

import (
	"strings"
)

// IsEmptyDocument checks if a yaml document is empty (contains only comments or separators)
func IsEmptyDocument(document document) bool {
	for _, line := range strings.Split(string(document), "\n") {
		line := strings.TrimSpace(line)
		// A line is considered to contain actual content if it is not empty,
		// does not start with a comment (#), and is not a document separator (---)
		if line != "" && !strings.HasPrefix(line, "#") && line != "---" {
			return false
		}
	}
	return true
}
