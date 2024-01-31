package path

import (
	"path/filepath"
)

func GetFullPath(path string, basePath string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(basePath, path)
	} else {
		return path
	}
}

func GetFullPaths(paths []string, basePath string, git bool) []string {
	if git {
		return paths
	}
	var out []string
	for _, path := range paths {
		out = append(out, GetFullPath(path, basePath))
	}
	return out
}
