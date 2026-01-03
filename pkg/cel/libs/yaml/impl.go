package yaml

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (i *impl) parse(yamlObj ref.Val, value ref.Val) ref.Val {
	if y, err := utils.ConvertToNative[Yaml](yamlObj); err != nil {
		return types.WrapErr(err)
	} else if value, err := utils.ConvertToNative[string](value); err != nil {
		return types.WrapErr(err)
	} else {
		if result, err := y.Parse([]byte(value)); err != nil {
			return types.WrapErr(err)
		} else {
			return i.NativeToValue(result)
		}
	}
}
