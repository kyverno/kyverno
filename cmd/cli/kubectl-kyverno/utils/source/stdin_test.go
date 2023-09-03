package source

import "testing"

func TestIsStdin(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{{
		name: "default",
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStdin(); got != tt.want {
				t.Errorf("IsInputFromPipe() = %v, want %v", got, tt.want)
			}
		})
	}
}
