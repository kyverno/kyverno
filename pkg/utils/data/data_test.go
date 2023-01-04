package data

import (
	"reflect"
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

func TestToMap(t *testing.T) {
	type data struct {
		Dummy string
	}
	type args struct {
		data interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "with map[string]interface{}",
			args: args{
				data: map[string]interface{}{},
			},
			want:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "with string",
			args: args{
				data: "foo",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "with nil",
			args: args{
				data: nil,
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "with struct",
			args: args{
				data: data{Dummy: "foo"},
			},
			want: map[string]interface{}{
				"Dummy": "foo",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToMap(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCopySliceOfMaps(t *testing.T) {
	originalMap := map[string]interface{}{
		"rsc": 3711,
		"r":   2138,
		"gri": 1908,
		"adg": 912,
	}
	type args struct {
		s []map[string]interface{}
	}
	tests := []struct {
		name string
		args args
		want []interface{}
	}{
		{
			name: "with nil",
			args: args{
				s: nil,
			},
			want: nil,
		},
		{
			name: "with empty",
			args: args{
				s: []map[string]interface{}{},
			},
			want: []interface{}{},
		},
		{
			name: "with data",
			args: args{
				s: []map[string]interface{}{originalMap},
			},
			want: []interface{}{originalMap},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CopySliceOfMaps(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CopySliceOfMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}
