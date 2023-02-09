package anchor

import (
	"reflect"
	"testing"
)

func TestRemoveAnchorsFromPath(t *testing.T) {
	tests := []struct {
		name string
		str  string
		want string
	}{{
		str:  "/path/(to)/X(anchors)",
		want: "/path/to/anchors",
	}, {
		str:  "path/(to)/X(anchors)",
		want: "path/to/anchors",
	}, {
		str:  "../(to)/X(anchors)",
		want: "../to/anchors",
	}, {
		str:  "/path/(to)/X(anchors)",
		want: "/path/to/anchors",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveAnchorsFromPath(tt.str); got != tt.want {
				t.Errorf("RemoveAnchorsFromPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAnchorsResourcesFromMap(t *testing.T) {
	tests := []struct {
		name          string
		patternMap    map[string]interface{}
		wantAnchors   map[string]interface{}
		wantResources map[string]interface{}
	}{{
		patternMap: map[string]interface{}{
			"spec": "test",
		},
		wantAnchors: map[string]interface{}{},
		wantResources: map[string]interface{}{
			"spec": "test",
		},
	}, {
		patternMap: map[string]interface{}{
			"(spec)": "test",
		},
		wantAnchors: map[string]interface{}{
			"(spec)": "test",
		},
		wantResources: map[string]interface{}{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anchors, resources := GetAnchorsResourcesFromMap(tt.patternMap)
			if !reflect.DeepEqual(anchors, tt.wantAnchors) {
				t.Errorf("GetAnchorsResourcesFromMap() anchors = %v, want %v", anchors, tt.wantAnchors)
			}
			if !reflect.DeepEqual(resources, tt.wantResources) {
				t.Errorf("GetAnchorsResourcesFromMap() resources = %v, want %v", resources, tt.wantResources)
			}
		})
	}
}

func Test_resourceHasValueForKey(t *testing.T) {
	type args struct {
		resource interface{}
		key      string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			resource: map[string]interface{}{
				"spec": 123,
			},
			key: "spec",
		},
		want: true,
	}, {
		args: args{
			resource: map[string]interface{}{
				"spec": 123,
			},
			key: "metadata",
		},
		want: false,
	}, {
		args: args{
			resource: []interface{}{1, 2, 3},
			key:      "spec",
		},
		want: false,
	}, {
		args: args{
			resource: []interface{}{
				map[string]interface{}{
					"spec": 123,
				},
			},
			key: "spec",
		},
		want: true,
	}, {
		args: args{
			resource: 123,
			key:      "spec",
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceHasValueForKey(tt.args.resource, tt.args.key); got != tt.want {
				t.Errorf("resourceHasValueForKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
