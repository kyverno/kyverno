package context

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (c *impl) get_configmap_string_string(args ...ref.Val) ref.Val {
	if self, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if namespace, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if name, err := utils.ConvertToNative[string](args[2]); err != nil {
		return types.WrapErr(err)
	} else {
		cm, err := self.GetConfigMap(namespace, name)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to get configmap: %v", err)
		}
		return c.NativeToValue(cm.UnstructuredContent())
	}
}

func (c *impl) get_globalreference_string(ctx ref.Val, name ref.Val) ref.Val {
	if self, err := utils.ConvertToNative[Context](ctx); err != nil {
		return types.WrapErr(err)
	} else if name, err := utils.ConvertToNative[string](name); err != nil {
		return types.WrapErr(err)
	} else {
		globalRef, err := self.GetGlobalReference(name)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to get global reference: %v", err)
		}
		return c.NativeToValue(globalRef)
	}
}

func (c *impl) get_imagedata_string(ctx ref.Val, image ref.Val) ref.Val {
	if self, err := utils.ConvertToNative[Context](ctx); err != nil {
		return types.WrapErr(err)
	} else if image, err := utils.ConvertToNative[string](image); err != nil {
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
