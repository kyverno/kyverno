package resource

import (
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

func UnpackData(data map[string]any) (map[string]any, error) {
	if data == nil {
		return nil, nil
	}

	unpacked := make(map[string]any, len(data))
	for k, v := range data {
		val, err := UnpackDyn(v)
		if err != nil {
			return nil, err
		}
		unpacked[k] = val
	}

	return unpacked, nil
}

func UnpackDyn(data any) (any, error) {
	if data == nil {
		return nil, nil
	}

	switch value := data.(type) {
	case map[ref.Val]ref.Val:
		unpacked := make(map[string]any, len(value))
		for k, v := range value {
			key, err := utils.ConvertToNative[string](k)
			if err != nil {
				return nil, err
			}
			val, err := UnpackDyn(v)
			if err != nil {
				return nil, err
			}
			unpacked[key] = val
		}

		return unpacked, nil
	case []ref.Val:
		unpacked := make([]any, len(value))
		for i, v := range value {
			val, err := UnpackDyn(v)
			if err != nil {
				return nil, err
			}
			unpacked[i] = val
		}

		return unpacked, nil
	case ref.Val:
		return UnpackDyn(value.Value())
	}

	return data, nil
}
