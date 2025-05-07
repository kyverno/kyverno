package globalcontext

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (c *impl) get_string(context ref.Val, name ref.Val) ref.Val {
	if context, err := utils.ConvertToNative[Context](context); err != nil {
		return types.WrapErr(err)
	} else if name, err := utils.ConvertToNative[string](name); err != nil {
		return types.WrapErr(err)
	} else {
		globalRef, err := context.GetGlobalReference(name, "")
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to get global reference: %v", err)
		}
		return c.NativeToValue(globalRef)
	}
}

func (c *impl) get_string_string(args ...ref.Val) ref.Val {
	if len(args) != 3 {
		return types.NewErr("expected 3 arguments, got %d", len(args))
	}
	if self, err := utils.ConvertToNative[Context](args[0]); err != nil {
		return types.WrapErr(err)
	} else if name, err := utils.ConvertToNative[string](args[1]); err != nil {
		return types.WrapErr(err)
	} else if projection, err := utils.ConvertToNative[string](args[2]); err != nil {
		return types.WrapErr(err)
	} else {
		globalRef, err := self.GetGlobalReference(name, projection)
		if err != nil {
			// Errors are not expected here since Parse is a more lenient parser than ParseRequestURI.
			return types.NewErr("failed to get global reference: %v", err)
		}
		return c.NativeToValue(globalRef)
	}
}
