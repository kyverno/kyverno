package matching

import (
	"github.com/kyverno/kyverno/pkg/cel/compiler"
)

func MatchImage(image string, c ...compiler.MatchImageReference) (bool, error) {
	if len(c) == 0 {
		return true, nil
	}
	for _, v := range c {
		if matched, err := v.Match(image); err != nil {
			return false, err
		} else if matched {
			return true, nil
		}
	}
	return false, nil
}
