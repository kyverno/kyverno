package yaml

import (
	"strings"
)

// IsEmptyDocument checks if a yaml document is empty (contains only comments
// and YAML document boundary markers).
func IsEmptyDocument(document document) bool {
	for _, line := range strings.Split(string(document), "\n") {
		line := strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Accept --- and ... markers optionally followed by whitespace and/or a comment.
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "...") {
			rest := strings.TrimSpace(line[3:])
			if rest == "" || strings.HasPrefix(rest, "#") {
				continue
			}
		}
		return false
	}
	return true
}
