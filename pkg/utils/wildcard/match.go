package wildcard

import (
	wildcard "github.com/IGLOU-EU/go-wildcard"
)

func Match(pattern, name string) bool {
	return wildcard.Match(pattern, name)
}
