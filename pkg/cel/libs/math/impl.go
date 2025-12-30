package math

import (
	"math"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (c *impl) round(val ref.Val, prec ref.Val) ref.Val {
	v, err := utils.ConvertToNative[float64](val)
	if err != nil {
		return types.WrapErr(err)
	}
	p, err := utils.ConvertToNative[int64](prec)
	if err != nil {
		return types.WrapErr(err)
	}
	shift := math.Pow(10, float64(p))
	return c.NativeToValue(math.Round(v*shift) / shift)
}
