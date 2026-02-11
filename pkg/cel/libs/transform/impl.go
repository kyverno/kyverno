package transform

import (
	"fmt"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (c *impl) list_of_objects_to_map(args ...ref.Val) ref.Val {
	if len(args) < 4 {
		return types.WrapErr(fmt.Errorf("listObjToMap must take 4 arguments, %d were passed", len(args)))
	}
	list1, err := utils.ConvertToNative[[]any](args[0])
	if err != nil {
		return types.WrapErr(err)
	}
	list2, err := utils.ConvertToNative[[]any](args[1])
	if err != nil {
		return types.WrapErr(err)
	}
	keyName, err := utils.ConvertToNative[string](args[2])
	if err != nil {
		return types.WrapErr(err)
	}
	valueName, err := utils.ConvertToNative[string](args[3])
	if err != nil {
		return types.WrapErr(err)
	}
	if len(list1) != len(list2) {
		return types.WrapErr(fmt.Errorf("listObjToMap must take lists of equal length. list1: %d, list2: %d", len(list1), len(list2)))
	}
	ret := make(map[string]any)
	for i, entry := range list1 {
		var (
			entry1 map[string]any
			entry2 map[string]any
			ok     bool
		)

		// attempt to handle the entry first as a map string any, if it failed try as a map of ref.Val to ref.Val
		entry1, ok = entry.(map[string]any)
		if !ok {
			entry1, err = refValMapToGoMap(entry.(map[ref.Val]ref.Val))
			if err != nil {
				return types.WrapErr(err)
			}
		}

		k, ok := entry1[keyName].(string)
		if !ok {
			return types.WrapErr(fmt.Errorf("the passed key name cannot be handled as a string in the key object list"))
		}

		entry2, ok = list2[i].(map[string]any)
		if !ok {
			entry2, err = refValMapToGoMap(list2[i].(map[ref.Val]ref.Val))
			if err != nil {
				return types.WrapErr(fmt.Errorf("object cannot be handled as a map string to any in the value object list"))
			}
		}

		ret[k] = entry2[valueName]
	}
	return c.NativeToValue(ret)
}

func refValMapToGoMap(valMap map[ref.Val]ref.Val) (map[string]any, error) {
	resultMap := make(map[string]any, len(valMap))
	for k, v := range valMap {
		keyStr, err := utils.ConvertToNative[string](k)
		if err != nil {
			return nil, fmt.Errorf("failed to convert key to a string, %+v", k.Type().TypeName())
		}
		valAny, err := utils.ConvertToNative[any](v)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value to any, %+v", v.Type().TypeName())
		}
		resultMap[keyStr] = valAny
	}
	return resultMap, nil
}
