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
		// A comment must be separated from the marker by whitespace, otherwise the line
		// is treated as content (e.g. "---#foo" is not a marker with a trailing comment).
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "...") {
			rest := line[3:]
			if rest == "" {
				continue
			}
			if strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t") {
				rest = strings.TrimSpace(rest)
				if rest == "" || strings.HasPrefix(rest, "#") {
					continue
				}
			}
		}
		return false
	}
	return true
}
