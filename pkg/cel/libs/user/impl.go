package user

import (
	"strings"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

const (
	saPrefix = "system:serviceaccount:"
)

type impl struct {
	types.Adapter
}

func (c *impl) parse_service_account_string(user ref.Val) ref.Val {
	if user, err := utils.ConvertToNative[string](user); err != nil {
		return types.WrapErr(err)
	} else {
		var sa ServiceAccount
		if strings.HasPrefix(user, saPrefix) {
			user = user[len(saPrefix):]
			if sep := strings.Index(user, ":"); sep != -1 {
				sa.Namespace = user[:sep]
				sa.Name = user[sep+1:]
			}
		}
		return c.NativeToValue(sa)
	}
}
