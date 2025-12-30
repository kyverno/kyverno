package json

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (i *impl) unmarshal(json ref.Val, value ref.Val) ref.Val {
	if json, err := utils.ConvertToNative[Json](json); err != nil {
		return types.WrapErr(err)
	} else if value, err := utils.ConvertToNative[string](value); err != nil {
		return types.WrapErr(err)
	} else {
		if value, err := json.Unmarshal([]byte(value)); err != nil {
			return types.WrapErr(err)
		} else {
			return i.NativeToValue(value)
		}
	}
}
