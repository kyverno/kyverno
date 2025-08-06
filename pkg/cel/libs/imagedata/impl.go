package imagedata

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (c *impl) get_imagedata_string(args ...ref.Val) ref.Val {
	if len(args) != 2 {
		return types.NewErr("expected 2 arguments, got %d", len(args))
	}
	if self, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if image, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else {
		globalRef, err := self.GetImageData(image)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to get image data: %v", err)
		}
		return c.NativeToValue(globalRef)
	}
}
