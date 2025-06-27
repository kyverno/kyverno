package utils

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

func GetArg[T any](args []ref.Val, index int) (T, ref.Val) {
	if out, err := ConvertToNative[T](args[index]); err != nil {
		return out, types.NewErr("invalid arg %d: %v", index, err)
	} else {
		return out, nil
	}
}
