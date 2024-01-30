package file

import (
	"testing"
)

func TestIsYaml(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{{
		name: "empty",
		path: "",
		want: false,
	}, {
		name: "yaml",
		path: "something.yaml",
		want: true,
	}, {
		name: "yml",
		path: "something.yml",
		want: true,
	}, {
		name: "json",
		path: "something.json",
		want: false,
	}, {
		name: "pdf",
		path: "something.pdf",
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsYaml(tt.path); got != tt.want {
				t.Errorf("IsYaml() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsJson(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{{
		name: "empty",
		path: "",
		want: false,
	}, {
		name: "yaml",
		path: "something.yaml",
		want: false,
	}, {
		name: "yml",
		path: "something.yml",
		want: false,
	}, {
		name: "json",
		path: "something.json",
		want: true,
	}, {
		name: "pdf",
		path: "something.pdf",
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJson(tt.path); got != tt.want {
				t.Errorf("IsJson() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsYamlOrJson(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{{
		name: "empty",
		path: "",
		want: false,
	}, {
		name: "yaml",
		path: "something.yaml",
		want: true,
	}, {
		name: "yml",
		path: "something.yml",
		want: true,
	}, {
		name: "json",
		path: "something.json",
		want: true,
	}, {
		name: "pdf",
		path: "something.pdf",
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsYamlOrJson(tt.path); got != tt.want {
				t.Errorf("IsYamlOrJson() = %v, want %v", got, tt.want)
			}
		})
	}
}
