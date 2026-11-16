package source

import "testing"

func TestIsOCI(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{{
		name: "empty",
		in:   "",
		want: false,
	}, {
		name: "oci prefix",
		in:   "oci://ghcr.io/org/repo:v1",
		want: true,
	}, {
		name: "oci prefix with digest",
		in:   "oci://ghcr.io/org/repo@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		want: true,
	}, {
		name: "http url",
		in:   "http://github.com/kyverno/policies",
		want: false,
	}, {
		name: "local path",
		in:   "/oci/repo",
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOCI(tt.in); got != tt.want {
				t.Errorf("IsOCI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripOCIPrefix(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{{
		name: "with prefix",
		in:   "oci://ghcr.io/org/repo:v1",
		want: "ghcr.io/org/repo:v1",
	}, {
		name: "without prefix",
		in:   "ghcr.io/org/repo:v1",
		want: "ghcr.io/org/repo:v1",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripOCIPrefix(tt.in); got != tt.want {
				t.Errorf("StripOCIPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
