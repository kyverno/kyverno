package fileinfo

import (
	"io/fs"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeFileInfo implements fs.FileInfo for testing.
type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string        { return f.name }
func (f fakeFileInfo) Size() int64         { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode   { return 0 }
func (f fakeFileInfo) ModTime() time.Time  { return time.Time{} }
func (f fakeFileInfo) IsDir() bool         { return f.isDir }
func (f fakeFileInfo) Sys() interface{}    { return nil }

func TestIsYaml(t *testing.T) {
	tests := []struct {
		name string
		info fs.FileInfo
		want bool
	}{{
		name: "yaml file",
		info: fakeFileInfo{name: "policy.yaml", isDir: false},
		want: true,
	}, {
		name: "yml file",
		info: fakeFileInfo{name: "policy.yml", isDir: false},
		want: true,
	}, {
		name: "json file",
		info: fakeFileInfo{name: "policy.json", isDir: false},
		want: false,
	}, {
		name: "directory with yaml name",
		info: fakeFileInfo{name: "dir.yaml", isDir: true},
		want: false,
	}, {
		name: "no extension",
		info: fakeFileInfo{name: "Makefile", isDir: false},
		want: false,
	}, {
		name: "nested path yaml",
		info: fakeFileInfo{name: "path/to/file.yaml", isDir: false},
		want: true,
	}, {
		name: "uppercase extension",
		info: fakeFileInfo{name: "FILE.YAML", isDir: false},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsYaml(tt.info))
		})
	}
}

func TestIsJson(t *testing.T) {
	tests := []struct {
		name string
		info fs.FileInfo
		want bool
	}{{
		name: "json file",
		info: fakeFileInfo{name: "data.json", isDir: false},
		want: true,
	}, {
		name: "yaml file",
		info: fakeFileInfo{name: "data.yaml", isDir: false},
		want: false,
	}, {
		name: "directory with json name",
		info: fakeFileInfo{name: "dir.json", isDir: true},
		want: false,
	}, {
		name: "no extension",
		info: fakeFileInfo{name: "README", isDir: false},
		want: false,
	}, {
		name: "txt file",
		info: fakeFileInfo{name: "notes.txt", isDir: false},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsJson(tt.info))
		})
	}
}

func TestIsYamlOrJson(t *testing.T) {
	tests := []struct {
		name string
		info fs.FileInfo
		want bool
	}{{
		name: "yaml file",
		info: fakeFileInfo{name: "file.yaml", isDir: false},
		want: true,
	}, {
		name: "yml file",
		info: fakeFileInfo{name: "file.yml", isDir: false},
		want: true,
	}, {
		name: "json file",
		info: fakeFileInfo{name: "file.json", isDir: false},
		want: true,
	}, {
		name: "txt file",
		info: fakeFileInfo{name: "file.txt", isDir: false},
		want: false,
	}, {
		name: "directory",
		info: fakeFileInfo{name: "configs.yaml", isDir: true},
		want: false,
	}, {
		name: "go file",
		info: fakeFileInfo{name: "main.go", isDir: false},
		want: false,
	}, {
		name: "hidden yaml file",
		info: fakeFileInfo{name: ".hidden.yaml", isDir: false},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsYamlOrJson(tt.info))
		})
	}
}
