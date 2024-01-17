package source

import (
	"errors"
	"os"
	"testing"
)

func TestIsStdin(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{{
		name: "default",
		path: "-",
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStdin(tt.path); got != tt.want {
				t.Errorf("IsInputFromPipe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isStdin(t *testing.T) {
	tests := []struct {
		name   string
		stater func(*os.File) (os.FileInfo, error)
		want   bool
	}{{
		name:   "nil stater",
		stater: nil,
		want:   false,
	}, {
		name:   "default stater",
		stater: defaultStater,
		want:   false,
	}, {
		name: "error stater",
		stater: func(_ *os.File) (os.FileInfo, error) {
			return nil, errors.New("test")
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isStdin(tt.stater); got != tt.want {
				t.Errorf("isStdin() = %v, want %v", got, tt.want)
			}
		})
	}
}
