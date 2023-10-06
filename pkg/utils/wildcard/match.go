package wildcard

import (
	"strings"

	wildcard "github.com/IGLOU-EU/go-wildcard"
)

func Match(pattern, name string) bool {
	if strings.HasPrefix(pattern, "!") {
		return !Match(strings.TrimPrefix(pattern, "!"), name)
	}
	return wildcard.Match(pattern, name)
}
