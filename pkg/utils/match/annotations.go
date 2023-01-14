package match

import (
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
)

func CheckAnnotations(expected map[string]string, actual map[string]string) bool {
	if len(expected) == 0 {
		return true
	}
	for k, v := range expected {
		match := false
		for k1, v1 := range actual {
			if wildcard.Match(k, k1) && wildcard.Match(v, v1) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}
