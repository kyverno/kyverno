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
			return types.NewErr("failed to get resource: %v", err)
		}
		return c.NativeToValue(cm.UnstructuredContent())
	}
}
