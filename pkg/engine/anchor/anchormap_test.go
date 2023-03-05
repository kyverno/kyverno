package anchor

import (
	"reflect"
	"testing"
)

func TestNewAnchorMap(t *testing.T) {
	tests := []struct {
		name string
		want *AnchorMap
	}{{
		want: &AnchorMap{anchorMap: map[string]bool{}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewAnchorMap(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAnchorMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnchorMap_KeysAreMissing(t *testing.T) {
	type fields struct {
		anchorMap   map[string]bool
		AnchorError validateAnchorError
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		fields: fields{
			anchorMap: map[string]bool{},
		},
		want: false,
	}, {
		fields: fields{
			anchorMap: map[string]bool{
				"a": true,
				"b": false,
			},
		},
		want: true,
	}, {
		fields: fields{
			anchorMap: map[string]bool{
				"a": true,
				"b": true,
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &AnchorMap{
				anchorMap:   tt.fields.anchorMap,
				AnchorError: tt.fields.AnchorError,
			}
			if got := ac.KeysAreMissing(); got != tt.want {
				t.Errorf("AnchorMap.KeysAreMissing() = %v, want %v", got, tt.want)
			}
		})
	}
}
