package git

import (
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
)

// mockFileInfo implements fs.FileInfo for testing
type mockFileInfo struct {
	name  string
	isDir bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() fs.FileMode  { return 0644 }
func (m mockFileInfo) ModTime() time.Time { return time.Now() }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }

func TestIsYaml_YamlFiles(t *testing.T) {
	tests := []struct {
		name     string
		file     fs.FileInfo
		expected bool
	}{
		{
			name:     "yaml extension",
			file:     mockFileInfo{name: "policy.yaml", isDir: false},
			expected: true,
		},
		{
			name:     "yml extension",
			file:     mockFileInfo{name: "config.yml", isDir: false},
			expected: true,
		},
		{
			name:     "txt file",
			file:     mockFileInfo{name: "readme.txt", isDir: false},
			expected: false,
		},
		{
			name:     "go file",
			file:     mockFileInfo{name: "main.go", isDir: false},
			expected: false,
		},
		{
			name:     "no extension",
			file:     mockFileInfo{name: "Dockerfile", isDir: false},
			expected: false,
		},
		{
			name:     "empty name",
			file:     mockFileInfo{name: "", isDir: false},
			expected: false,
		},
		{
			name:     "directory with yaml name",
			file:     mockFileInfo{name: "config.yaml", isDir: true},
			expected: false,
		},
		{
			name:     "uppercase yaml",
			file:     mockFileInfo{name: "CONFIG.YAML", isDir: false},
			expected: false,
		},
		{
			name:     "hidden yaml file",
			file:     mockFileInfo{name: ".config.yaml", isDir: false},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsYaml(tt.file)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestListFiles_EmptyDirectory(t *testing.T) {
	memFS := memfs.New()
	err := memFS.MkdirAll("/empty", 0755)
	assert.NoError(t, err)

	files, err := ListFiles(memFS, "/empty", func(f fs.FileInfo) bool { return true })
	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestListFiles_SingleFile(t *testing.T) {
	memFS := memfs.New()
	err := memFS.MkdirAll("/test", 0755)
	assert.NoError(t, err)

	file, err := memFS.Create("/test/file.txt")
	assert.NoError(t, err)
	file.Close()

	files, err := ListFiles(memFS, "/test", func(f fs.FileInfo) bool { return true })
	assert.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0], "file.txt")
}

func TestListFiles_PredicateFiltering(t *testing.T) {
	memFS := memfs.New()
	err := memFS.MkdirAll("/test", 0755)
	assert.NoError(t, err)

	// Create yaml and txt files
	yamlFile, err := memFS.Create("/test/config.yaml")
	assert.NoError(t, err)
	yamlFile.Close()

	txtFile, err := memFS.Create("/test/readme.txt")
	assert.NoError(t, err)
	txtFile.Close()

	// List only yaml files
	files, err := ListFiles(memFS, "/test", IsYaml)
	assert.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0], "config.yaml")
}

func TestListFiles_NestedDirectories(t *testing.T) {
	memFS := memfs.New()

	// Create nested structure
	err := memFS.MkdirAll("/root/sub1/sub2", 0755)
	assert.NoError(t, err)

	// Create files at different levels
	file1, err := memFS.Create("/root/file1.txt")
	assert.NoError(t, err)
	file1.Close()

	file2, err := memFS.Create("/root/sub1/file2.txt")
	assert.NoError(t, err)
	file2.Close()

	file3, err := memFS.Create("/root/sub1/sub2/file3.txt")
	assert.NoError(t, err)
	file3.Close()

	// List all files recursively
	files, err := ListFiles(memFS, "/root", func(f fs.FileInfo) bool { return true })
	assert.NoError(t, err)
	assert.Len(t, files, 3)
}

func TestListFiles_NonexistentPath(t *testing.T) {
	memFS := memfs.New()

	files, err := ListFiles(memFS, "/nonexistent", func(f fs.FileInfo) bool { return true })
	assert.Error(t, err)
	assert.Nil(t, files)
}

func TestListFiles_PathCleaning(t *testing.T) {
	memFS := memfs.New()
	err := memFS.MkdirAll("/test", 0755)
	assert.NoError(t, err)

	file, err := memFS.Create("/test/file.txt")
	assert.NoError(t, err)
	file.Close()

	// Test with messy path
	files, err := ListFiles(memFS, "/test/./", func(f fs.FileInfo) bool { return true })
	assert.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestListYamls_FiltersYamlFiles(t *testing.T) {
	memFS := memfs.New()
	err := memFS.MkdirAll("/configs", 0755)
	assert.NoError(t, err)

	// Create mixed files
	yaml1, err := memFS.Create("/configs/app.yaml")
	assert.NoError(t, err)
	yaml1.Close()

	yml1, err := memFS.Create("/configs/db.yml")
	assert.NoError(t, err)
	yml1.Close()

	txt, err := memFS.Create("/configs/readme.txt")
	assert.NoError(t, err)
	txt.Close()

	// List yamls only
	yamls, err := ListYamls(memFS, "/configs")
	assert.NoError(t, err)
	assert.Len(t, yamls, 2)

	// Check that only yaml/yml files are returned
	for _, file := range yamls {
		assert.True(t,
			strings.Contains(file, ".yaml") || strings.Contains(file, ".yml"),
			"expected yaml or yml file, got: %s", file)
	}
}

func TestListYamls_EmptyDirectory(t *testing.T) {
	memFS := memfs.New()
	err := memFS.MkdirAll("/empty", 0755)
	assert.NoError(t, err)

	yamls, err := ListYamls(memFS, "/empty")
	assert.NoError(t, err)
	assert.Empty(t, yamls)
}

func TestListYamls_NestedYamlFiles(t *testing.T) {
	memFS := memfs.New()

	// Create nested structure with yaml files
	err := memFS.MkdirAll("/policies/prod/apps", 0755)
	assert.NoError(t, err)

	yaml1, err := memFS.Create("/policies/policy.yaml")
	assert.NoError(t, err)
	yaml1.Close()

	yaml2, err := memFS.Create("/policies/prod/config.yml")
	assert.NoError(t, err)
	yaml2.Close()

	yaml3, err := memFS.Create("/policies/prod/apps/deployment.yaml")
	assert.NoError(t, err)
	yaml3.Close()

	// Non-yaml file
	txt, err := memFS.Create("/policies/readme.md")
	assert.NoError(t, err)
	txt.Close()

	yamls, err := ListYamls(memFS, "/policies")
	assert.NoError(t, err)
	assert.Len(t, yamls, 3)
}

func TestListYamls_NonexistentPath(t *testing.T) {
	memFS := memfs.New()

	yamls, err := ListYamls(memFS, "/nonexistent")
	assert.Error(t, err)
	assert.Nil(t, yamls)
}

// TestListFiles_WithHiddenFiles verifies that hidden files are correctly handled
func TestListFiles_WithHiddenFiles(t *testing.T) {
	memFS := memfs.New()
	err := memFS.MkdirAll("/test", 0755)
	assert.NoError(t, err)

	// Create a hidden yaml file
	hidden, err := memFS.Create("/test/.hidden.yaml")
	assert.NoError(t, err)
	hidden.Close()

	// Create a regular yaml file
	regular, err := memFS.Create("/test/regular.yaml")
	assert.NoError(t, err)
	regular.Close()

	files, err := ListFiles(memFS, "/test", IsYaml)
	assert.NoError(t, err)
	assert.Len(t, files, 2, "should include both hidden and regular yaml files")
}
