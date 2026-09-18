package git

import (
	"io/fs"
	"testing"
	"time"
)

// fakeFileInfo lets us drive IsYaml without touching the filesystem.
type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

func TestIsYaml(t *testing.T) {
	cases := []struct {
		info fakeFileInfo
		want bool
	}{
		{fakeFileInfo{name: "policy.yaml"}, true},
		{fakeFileInfo{name: "policy.yml"}, true},
		{fakeFileInfo{name: "deeply/nested/POLICY.YAML"}, false}, // case-sensitive on purpose
		{fakeFileInfo{name: "policy.yaml.bak"}, false},
		{fakeFileInfo{name: "policy.json"}, false},
		{fakeFileInfo{name: "Makefile"}, false},
		{fakeFileInfo{name: ""}, false},
		// Even a perfectly-named directory must not be treated as a yaml file.
		{fakeFileInfo{name: "policies.yaml", isDir: true}, false},
	}
	for _, c := range cases {
		if got := IsYaml(c.info); got != c.want {
			t.Errorf("IsYaml(%q, dir=%v) = %v, want %v", c.info.name, c.info.isDir, got, c.want)
		}
	}
}
