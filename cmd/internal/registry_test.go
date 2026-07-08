package internal

import (
	"reflect"
	"testing"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "empty",
			in:   "",
			want: nil,
		},
		{
			name: "single value",
			in:   "secret",
			want: []string{"secret"},
		},
		{
			name: "whitespace only",
			in:   "   ",
			want: nil,
		},
		{
			name: "only separators",
			in:   ",,,",
			want: nil,
		},
		{
			name: "trims whitespace and drops empty values",
			in:   "secret-a, secret-b,, secret-c , ",
			want: []string{"secret-a", "secret-b", "secret-c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("splitAndTrim(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
