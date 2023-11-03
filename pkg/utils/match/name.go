package match

import (
	"github.com/kyverno/kyverno/ext/wildcard"
)

func CheckName(expected, actual string) bool {
	return wildcard.Match(expected, actual)
}
