package yaml

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
)

type mockYaml struct {
	ret any
	err error
}

func (m *mockYaml) Parse(b []byte) (any, error) {
	return m.ret, m.err
}

func TestImplParse(t *testing.T) {
	// Create a CEL environment with proper type registration
	env, err := cel.NewEnv(
		ext.NativeTypes(reflect.TypeFor[Yaml]()),
	)
	if err != nil {
		t.Fatalf("failed to create CEL environment: %v", err)
	}
	adapter := env.CELTypeAdapter()
	i := &impl{adapter}

	type testCase struct {
		name         string
		yamlVal      any
		valueVal     any
		expectErr    bool
		expectNull   bool
		expectResult any
	}

	tests := []testCase{
		{
			name: "success - nested yaml",
			yamlVal: Yaml{&mockYaml{ret: map[string]any{
				"species": map[string]any{
					"dog":       "lab",
					"name":      "dory",
					"isGoodBoi": false,
				},
				"snacks": []any{"chimken", "fries", "pizza"},
			}, err: nil}},
			valueVal: `
species:
  dog: lab
  name: dory
  isGoodBoi: false
snacks:
- chimken
- fries
- pizza
`,
			expectErr: false,
			expectResult: map[string]any{
				"species": map[string]any{
					"dog":       "lab",
					"name":      "dory",
					"isGoodBoi": false,
				},
				"snacks": []any{"chimken", "fries", "pizza"},
			},
		},
		{
			name:      "yaml convert error",
			yamlVal:   "not a yaml struct",
			valueVal:  "irrelevant",
			expectErr: true,
		},
		{
			name:      "value convert error (non-string)",
			yamlVal:   Yaml{&mockYaml{ret: nil, err: nil}},
			valueVal:  12345,
			expectErr: true,
		},
		{
			name:      "parse error",
			yamlVal:   Yaml{&mockYaml{ret: nil, err: errors.New("parse failed")}},
			valueVal:  "bad yaml",
			expectErr: true,
		},
		{
			name:      "yaml is nil",
			yamlVal:   nil,
			valueVal:  "foo: bar",
			expectErr: true,
		},
		{
			name:      "value is nil",
			yamlVal:   Yaml{&mockYaml{ret: map[string]any{"foo": "bar"}, err: nil}},
			valueVal:  nil,
			expectErr: true,
		},
		{
			name:       "value is empty string",
			yamlVal:    Yaml{&mockYaml{ret: nil, err: nil}},
			valueVal:   "",
			expectErr:  false,
			expectNull: true,
		},
		{
			name:         "yaml scalar value",
			yamlVal:      Yaml{&mockYaml{ret: 42, err: nil}},
			valueVal:     "42",
			expectErr:    false,
			expectResult: int64(42),
		},
		{
			name:         "yaml list only",
			yamlVal:      Yaml{&mockYaml{ret: []any{"a", "b", "c"}, err: nil}},
			valueVal:     "- a\n- b\n- c\n",
			expectErr:    false,
			expectResult: []any{"a", "b", "c"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yamlVal := adapter.NativeToValue(tc.yamlVal)
			valueVal := adapter.NativeToValue(tc.valueVal)
			result := i.parse(yamlVal, valueVal)
			if tc.expectErr {
				if result.Type() != types.ErrType {
					t.Errorf("expected error type, got %v (value=%v)", result.Type(), result)
				}
			} else if tc.expectNull {
				if result.Type() != types.NullType {
					t.Errorf("expected null type, got %v (value=%v)", result.Type(), result)
				}
			} else {
				// Convert the result back to native type for comparison
				got, err := result.ConvertToNative(reflect.TypeOf(tc.expectResult))
				if err != nil {
					t.Fatalf("unexpected conversion error: %v", err)
				}
				if !reflect.DeepEqual(got, tc.expectResult) {
					t.Errorf("expected result %v, got %v", tc.expectResult, got)
				}
			}
		})
	}
}
