package variables

import (
	"github.com/kyverno/go-jmespath"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

func CheckNotFoundErr(err error) bool {
	if err != nil {
		switch err.(type) {
		case jmespath.NotFoundError:
			return true
		case enginecontext.InvalidVariableError:
			return false
		default:
			return false
		}
	}

	return true
}
