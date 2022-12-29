package data

import (
	"testing"

	"gotest.tools/assert"
)

func TestOriginalMapMustNotBeChanged(t *testing.T) {
	// no variables
	originalMap := map[string]interface{}{
		"rsc": 3711,
		"r":   2138,
		"gri": 1908,
		"adg": 912,
	}
	mapCopy := CopyMap(originalMap)
	mapCopy["r"] = 1
	assert.Equal(t, originalMap["r"], 2138)
}

func TestSliceContains(t *testing.T) {
	type args struct {
		slice  []string
		values []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "empty slice",
		args: args{
			slice:  []string{},
			values: []string{"ccc", "ddd"},
		},
		want: false,
	}, {
		name: "nil slice",
		args: args{
			slice:  nil,
			values: []string{"ccc", "ddd"},
		},
		want: false,
	}, {
		name: "empty values",
		args: args{
			slice:  []string{"aaa", "bbb"},
			values: []string{},
		},
		want: false,
	}, {
		name: "nil values",
		args: args{
			slice:  []string{"aaa", "bbb"},
			values: nil,
		},
		want: false,
	}, {
		name: "none match",
		args: args{
			slice:  []string{"aaa", "bbb"},
			values: []string{"ccc", "ddd"},
		},
		want: false,
	}, {
		name: "one match",
		args: args{
			slice:  []string{"aaa", "bbb"},
			values: []string{"aaa"},
		},
		want: true,
	}, {
		name: "one match, one doesn't match",
		args: args{
			slice:  []string{"aaa", "bbb"},
			values: []string{"aaa", "ddd"},
		},
		want: true,
	}, {
		name: "all match",
		args: args{
			slice:  []string{"aaa", "bbb"},
			values: []string{"aaa", "bbb"},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SliceContains(tt.args.slice, tt.args.values...); got != tt.want {
				t.Errorf("SliceContains() = %v, want %v", got, tt.want)
			}
		})
	}
}
