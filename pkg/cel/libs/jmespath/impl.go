package jmespath

import (
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/sdk/extensions/cel/utils"
)

type jmesfuncs struct {
	types.Adapter
}

func (f *jmesfuncs) match_string_dyn(queryVal ref.Val, objVal ref.Val) ref.Val {
	query, err := utils.ConvertToNative[string](queryVal)
	if err != nil {
		return types.WrapErr(err)
	}

	result, err := gojmespath.Search(query, objVal.Value())
	if err != nil {
		return types.NewErr("jmespath evaluation failed: %v", err)
	}

	return f.NativeToValue(result)
}
