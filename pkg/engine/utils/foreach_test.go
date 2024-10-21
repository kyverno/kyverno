package utils

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
)

func Test_InvertElements(t *testing.T) {
	elems := []interface{}{"a", "b", "c"}
	elemsInverted := InvertElements(elems)

	assert.Equal(t, "a", elemsInverted[2])
	assert.Equal(t, "b", elemsInverted[1])
	assert.Equal(t, "c", elemsInverted[0])
}

func Test_EvaluateList(t *testing.T) {
	entryName := "test_object"
	cases := []struct {
		name     string
		rawData  []byte
		jmesPath string
		expected interface{}
	}{
		{
			name:     "slice data",
			rawData:  []byte(`["test-value-1", "test-value-2"]`),
			jmesPath: entryName,
			expected: []interface{}{"test-value-1", "test-value-2"},
		},
		{
			name: "map data",
			rawData: []byte(`
				{
					"test-key-1": "test-value-1",
					"test-key-2": "test-value-2"
				}
			`),
			jmesPath: entryName + ".items(@, 'key', 'value')",
			expected: []interface{}{
				map[string]interface{}{
					"key":   "test-key-1",
					"value": "test-value-1",
				},
				map[string]interface{}{
					"key":   "test-key-2",
					"value": "test-value-2",
				},
			},
		},
		{
			name: "map data with custom fields",
			rawData: []byte(`
				{
					"test-key-1": "test-value-1",
					"test-key-2": "test-value-2"
				}
			`),
			jmesPath: "test_object.items(@, 'another-key', 'another-value')",
			expected: []interface{}{
				map[string]interface{}{
					"another-key":   "test-key-1",
					"another-value": "test-value-1",
				},
				map[string]interface{}{
					"another-key":   "test-key-2",
					"another-value": "test-value-2",
				},
			},
		},
	}

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			ctx := context.NewContext(jp)
			assert.NoError(t, ctx.AddContextEntry(entryName, item.rawData))

			list, err := EvaluateList(item.jmesPath, ctx)
			assert.NoError(t, err)
			assert.Equal(t, item.expected, list)
		})
	}
}
