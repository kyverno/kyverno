package slices

import (
	"reflect"
	"strings"
	"testing"
)

func TestMap(t *testing.T) {
	tests := []struct {
		name     string
		source   []any
		cb       func(any) any
		expected []any
	}{
		{
			name:   "map strings to uppercase",
			source: []any{"a", "b", "c"},
			cb: func(i any) any {
				return strings.ToUpper(i.(string))
			},
			expected: []any{"A", "B", "C"},
		},
		{
			name:   "map integers to double",
			source: []any{1, 2, 3},
			cb: func(i any) any {
				return i.(int) * 2
			},
			expected: []any{2, 4, 6},
		},
		{
			name:   "map empty slice",
			source: []any{},
			cb: func(i any) any {
				return i
			},
			expected: []any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Map(tt.source, tt.cb)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Map() = %v, want %v", got, tt.expected)
			}
		})
	}
}
