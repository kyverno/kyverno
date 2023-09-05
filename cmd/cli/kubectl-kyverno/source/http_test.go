package source

import "testing"

func TestIsHttp(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{{
		name: "empty",
		in:   "",
		want: false,
	}, {
		name: "http",
		in:   "http://github.com/kyverno/policies",
		want: true,
	}, {
		name: "https",
		in:   "https://github.com/kyverno/policies",
		want: true,
	}, {
		name: "local path",
		in:   "/github.com/kyverno/policies",
		want: false,
	}, {
		name: "local path",
		in:   "/https/kyverno/policies",
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHttp(tt.in); got != tt.want {
				t.Errorf("IsHttp() = %v, want %v", got, tt.want)
			}
		})
	}
}
