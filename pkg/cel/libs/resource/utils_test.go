package resource

import (
	"testing"

	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
)

func TestUnpack(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	adapter := base.CELTypeAdapter()

	data := map[string]any{
		"key1": adapter.NativeToValue("value1"),
		"key2": map[ref.Val]ref.Val{
			adapter.NativeToValue("map1"): adapter.NativeToValue("mapValue1"),
			adapter.NativeToValue("map2"): adapter.NativeToValue(map[ref.Val]ref.Val{
				adapter.NativeToValue("nested1"): adapter.NativeToValue("val"),
			}),
		},
		"key3": []ref.Val{
			adapter.NativeToValue("listItem1"),
			adapter.NativeToValue("listItem2"),
			adapter.NativeToValue("listItem3"),
		},
	}

	unpacked, err := UnpackData(data)
	if !assert.NoError(t, err) {
		return
	}

	expected := map[string]any{
		"key1": "value1",
		"key2": map[string]any{
			"map1": "mapValue1",
			"map2": map[string]any{
				"nested1": "val",
			},
		},
		"key3": []any{
			"listItem1",
			"listItem2",
			"listItem3",
		},
	}

	assert.Equal(t, expected, unpacked)
}
