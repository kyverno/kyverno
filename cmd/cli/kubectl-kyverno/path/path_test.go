package path

// import (
// 	"reflect"
// 	"testing"
// )

// func TestGetFullPath(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		path     string
// 		basePath string
// 		want     string
// 	}{
// 		{
// 			name:     "relative",
// 			path:     "abc",
// 			basePath: "def",
// 			want:     "def/abc",
// 		},
// 		{
// 			name:     "absolute",
// 			path:     "/abc",
// 			basePath: "def",
// 			want:     "/abc",
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := GetFullPath(tt.path, tt.basePath); got != tt.want {
// 				t.Errorf("GetFullPath() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

// func TestGetFullPaths(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		paths    []string
// 		Paths    []string
// 		basePath string
// 		git      bool
// 		want     []string
// 	}{
// 		{
// 			name:     "relative (non git)",
// 			paths:    []string{"abc", "xyz"},
// 			Paths:    []string{"efg", "pqr"},
// 			basePath: "def",
// 			git:      false,
// 			want:     []string{"def/abc", "def/xyz", "def/efg", "def/pqr"},
// 		},
// 		{
// 			name:     "absolute (non git)",
// 			paths:    []string{"/abc", "/xyz"},
// 			Paths:    []string{"/efg", "/pqr"},
// 			basePath: "def",
// 			git:      false,
// 			want:     []string{"/abc", "/xyz", "/efg", "/pqr"},
// 		},
// 		{
// 			name:     "mixed (non git)",
// 			paths:    []string{"/abc", "xyz"},
// 			Paths:    []string{"/efg", "pqr"},
// 			basePath: "def",
// 			git:      false,
// 			want:     []string{"/abc", "def/xyz", "/efg", "def/pqr"},
// 		},
// 		{
// 			name:     "relative (git)",
// 			paths:    []string{"abc", "xyz"},
// 			Paths:    []string{"efg", "pqr"},
// 			basePath: "def",
// 			git:      true,
// 			want:     []string{"abc", "xyz", "efg", "pqr"},
// 		},
// 		{
// 			name:     "absolute (git)",
// 			paths:    []string{"/abc", "/xyz"},
// 			Paths:    []string{"/efg", "/pqr"},
// 			basePath: "def",
// 			git:      true,
// 			want:     []string{"/abc", "/xyz", "/efg", "/pqr"},
// 		},
// 		{
// 			name:     "mixed (git)",
// 			paths:    []string{"/abc", "xyz"},
// 			Paths:    []string{"/efg", "pqr"},
// 			basePath: "def",
// 			git:      true,
// 			want:     []string{"/abc", "xyz", "/efg", "pqr"},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := GetFullPaths(tt.paths, tt.Paths, tt.basePath, tt.git); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("GetFullPaths() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
