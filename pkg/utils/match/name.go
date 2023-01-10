package match

import (
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
)

func CheckName(name, resourceName string) bool {
	return wildcard.Match(name, resourceName)
}
