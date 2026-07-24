package fileinfo

import (
	"io/fs"
	"testing"
	"time"
)

type mockFileInfo struct {
	name  string
	isDir bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() fs.FileMode  { return 0 }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }

func TestIsYaml(t *testing.T) {
	tests := []struct {
		name string
		info fs.FileInfo
		want bool
	}{
		{name: "yaml file", info: mockFileInfo{name: "policy.yaml"}, want: true},
		{name: "yml file", info: mockFileInfo{name: "policy.yml"}, want: true},
		{name: "json file", info: mockFileInfo{name: "policy.json"}, want: false},
		{name: "text file", info: mockFileInfo{name: "readme.txt"}, want: false},
		{name: "directory with yaml name", info: mockFileInfo{name: "policy.yaml", isDir: true}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsYaml(tt.info); got != tt.want {
				t.Errorf("IsYaml() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsJson(t *testing.T) {
	tests := []struct {
		name string
		info fs.FileInfo
		want bool
	}{
		{name: "json file", info: mockFileInfo{name: "policy.json"}, want: true},
		{name: "yaml file", info: mockFileInfo{name: "policy.yaml"}, want: false},
		{name: "text file", info: mockFileInfo{name: "readme.txt"}, want: false},
		{name: "directory with json name", info: mockFileInfo{name: "policy.json", isDir: true}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJson(tt.info); got != tt.want {
				t.Errorf("IsJson() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsYamlOrJson(t *testing.T) {
	tests := []struct {
		name string
		info fs.FileInfo
		want bool
	}{
		{name: "yaml file", info: mockFileInfo{name: "policy.yaml"}, want: true},
		{name: "yml file", info: mockFileInfo{name: "policy.yml"}, want: true},
		{name: "json file", info: mockFileInfo{name: "policy.json"}, want: true},
		{name: "text file", info: mockFileInfo{name: "readme.txt"}, want: false},
		{name: "directory", info: mockFileInfo{name: "policy.yaml", isDir: true}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsYamlOrJson(tt.info); got != tt.want {
				t.Errorf("IsYamlOrJson() = %v, want %v", got, tt.want)
			}
		})
	}
}
