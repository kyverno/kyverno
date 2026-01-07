package json

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
)

type mockJson struct {
	ret any
	err error
}

func (m *mockJson) Unmarshal(b []byte) (any, error) {
	return m.ret, m.err
}

func TestImplUnmarshal(t *testing.T) {
	// Create a CEL environment with proper type registration
	env, err := cel.NewEnv(
		ext.NativeTypes(reflect.TypeFor[Json]()),
	)
	if err != nil {
		t.Fatalf("failed to create CEL environment: %v", err)
	}
	adapter := env.CELTypeAdapter()
	i := &impl{adapter}

	type testCase struct {
		name         string
		jsonVal      any
		valueVal     any
		expectErr    bool
		expectResult any
	}

	tests := []testCase{
		{
			name:         "success",
			jsonVal:      Json{&mockJson{ret: map[string]any{"foo": "bar"}, err: nil}},
			valueVal:     `{"foo":"bar"}`,
			expectErr:    false,
			expectResult: map[string]any{"foo": "bar"},
		},
		{
			name:      "json convert error",
			jsonVal:   "not a json struct",
			valueVal:  "irrelevant",
			expectErr: true,
		},
		{
			name:      "value convert error",
			jsonVal:   Json{&mockJson{ret: nil, err: nil}},
			valueVal:  12345,
			expectErr: true,
		},
		{
			name:      "unmarshal error",
			jsonVal:   Json{&mockJson{ret: nil, err: errors.New("unmarshal failed")}},
			valueVal:  "bad json",
			expectErr: true,
		},
		{
			name:      "json is nil",
			jsonVal:   nil,
			valueVal:  `{"foo":"bar"}`,
			expectErr: true,
		},
		{
			name:      "value is nil",
			jsonVal:   Json{&mockJson{ret: map[string]any{"foo": "bar"}, err: nil}},
			valueVal:  nil,
			expectErr: true,
		},
		{
			name:         "value is empty string",
			jsonVal:      Json{&mockJson{ret: map[string]any{}, err: nil}},
			valueVal:     "",
			expectErr:    false,
			expectResult: map[string]any{},
		},
		{
			name:         "json is empty struct",
			jsonVal:      Json{&mockJson{ret: map[string]any{}, err: nil}},
			valueVal:     `{"foo":"bar"}`,
			expectErr:    false,
			expectResult: map[string]any{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jsonVal := adapter.NativeToValue(tc.jsonVal)
			valueVal := adapter.NativeToValue(tc.valueVal)
			result := i.unmarshal(jsonVal, valueVal)
			if tc.expectErr {
				if result.Type() != types.ErrType {
					t.Errorf("expected error type, got %v", result.Type())
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
