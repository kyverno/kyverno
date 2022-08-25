package wildcard

import (
	wildcard "github.com/kyverno/go-wildcard"
)

func Match(pattern, name string) bool {
	return wildcard.Match(pattern, name)
}
