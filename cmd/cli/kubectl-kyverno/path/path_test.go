package path

import (
	"reflect"
	"testing"
)

func TestGetFullPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		basePath string
		want     string
	}{
		{
			name:     "relative",
			path:     "abc",
			basePath: "def",
			want:     "def/abc",
		},
		{
			name:     "absolute",
			path:     "/abc",
			basePath: "def",
			want:     "/abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetFullPath(tt.path, tt.basePath); got != tt.want {
				t.Errorf("GetFullPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFullPaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		basePath string
		git      bool
		want     []string
	}{
		{
			name:     "relative (non git)",
			paths:    []string{"abc", "xyz"},
			basePath: "def",
			git:      false,
			want:     []string{"def/abc", "def/xyz"},
		},
		{
			name:     "absolute (non git)",
			paths:    []string{"/abc", "/xyz"},
			basePath: "def",
			git:      false,
			want:     []string{"/abc", "/xyz"},
		},
		{
			name:     "mixed (non git)",
			paths:    []string{"/abc", "xyz"},
			basePath: "def",
			git:      false,
			want:     []string{"/abc", "def/xyz"},
		},
		{
			name:     "relative (git)",
			paths:    []string{"abc", "xyz"},
			basePath: "def",
			git:      true,
			want:     []string{"abc", "xyz"},
		},
		{
			name:     "absolute (git)",
			paths:    []string{"/abc", "/xyz"},
			basePath: "def",
			git:      true,
			want:     []string{"/abc", "/xyz"},
		},
		{
			name:     "mixed (git)",
			paths:    []string{"/abc", "xyz"},
			basePath: "def",
			git:      true,
			want:     []string{"/abc", "xyz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetFullPaths(tt.paths, tt.basePath, tt.git); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetFullPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}
