package variables

import (
	"errors"
	"strings"

	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

func CheckNotFoundErr(err error) bool {
	if err != nil {
		if strings.Contains(err.Error(), "Unknown key") {
			return true
		}
		if errors.As(err, &enginecontext.InvalidVariableError{}) {
			return false
		}
		return false
	}
	return true
}
