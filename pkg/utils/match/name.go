package match

import (
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
)

func CheckName(expected, actual string) bool {
	return wildcard.Match(expected, actual)
}
