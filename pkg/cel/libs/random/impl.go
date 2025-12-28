package random

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	regen "github.com/zach-klippenstein/goregen"
)

type impl struct {
	types.Adapter
}

func (c *impl) random(arg ref.Val) ref.Val {
	if value, err := utils.ConvertToNative[string](arg); err != nil {
		return types.WrapErr(err)
	} else {
		out, err := regen.Generate(value)
		if err != nil {
			return types.WrapErr(err)
		}
		return c.NativeToValue(out)
	}
}

func (c *impl) random_default_expr(vals ...ref.Val) ref.Val {
	out, err := regen.Generate("[0-9a-z]{8}")
	if err != nil {
		return types.WrapErr(err)
	}
	return c.NativeToValue(out)
}
