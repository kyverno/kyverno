package git

import (
	"io/fs"
	"testing"
	"time"
)

// mockFileInfo implements fs.FileInfo for testing
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
		name     string
		fileInfo fs.FileInfo
		want     bool
	}{
		{
			name:     "yaml extension",
			fileInfo: mockFileInfo{name: "policy.yaml", isDir: false},
			want:     true,
		},
		{
			name:     "yml extension",
			fileInfo: mockFileInfo{name: "config.yml", isDir: false},
			want:     true,
		},
		{
			name:     "json extension",
			fileInfo: mockFileInfo{name: "data.json", isDir: false},
			want:     false,
		},
		{
			name:     "no extension",
			fileInfo: mockFileInfo{name: "Makefile", isDir: false},
			want:     false,
		},
		{
			name:     "directory with yaml-like name",
			fileInfo: mockFileInfo{name: "policies.yaml", isDir: true},
			want:     false,
		},
		{
			name:     "directory with yml-like name",
			fileInfo: mockFileInfo{name: "configs.yml", isDir: true},
			want:     false,
		},
		{
			name:     "uppercase YAML extension",
			fileInfo: mockFileInfo{name: "Policy.YAML", isDir: false},
			want:     false,
		},
		{
			name:     "uppercase YML extension",
			fileInfo: mockFileInfo{name: "Config.YML", isDir: false},
			want:     false,
		},
		{
			name:     "mixed case yaml extension",
			fileInfo: mockFileInfo{name: "test.Yaml", isDir: false},
			want:     false,
		},
		{
			name:     "hidden yaml file",
			fileInfo: mockFileInfo{name: ".hidden.yaml", isDir: false},
			want:     true,
		},
		{
			name:     "hidden yml file",
			fileInfo: mockFileInfo{name: ".config.yml", isDir: false},
			want:     true,
		},
		{
			name:     "double extension ending in yaml",
			fileInfo: mockFileInfo{name: "file.tar.yaml", isDir: false},
			want:     true,
		},
		{
			name:     "yaml in middle of filename",
			fileInfo: mockFileInfo{name: "yaml-config.txt", isDir: false},
			want:     false,
		},
		{
			name:     "empty filename",
			fileInfo: mockFileInfo{name: "", isDir: false},
			want:     false,
		},
		{
			name:     "only extension yaml",
			fileInfo: mockFileInfo{name: ".yaml", isDir: false},
			want:     true,
		},
		{
			name:     "only extension yml",
			fileInfo: mockFileInfo{name: ".yml", isDir: false},
			want:     true,
		},
		{
			name:     "go file",
			fileInfo: mockFileInfo{name: "main.go", isDir: false},
			want:     false,
		},
		{
			name:     "markdown file",
			fileInfo: mockFileInfo{name: "README.md", isDir: false},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsYaml(tt.fileInfo)
			if got != tt.want {
				t.Errorf("IsYaml(%q, isDir=%v) = %v, want %v",
					tt.fileInfo.Name(), tt.fileInfo.IsDir(), got, tt.want)
			}
		})
	}
}
