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
