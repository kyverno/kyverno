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

func GetFullPaths(paths []string, Paths []string, basePath string, git bool) []string {
	if git {
		return paths
	}
	var out []string
	for _, path := range paths {
		out = append(out, GetFullPath(path, basePath))
	}
	if len(Paths) > 0 {
		for _, Path := range Paths {
			out = append(out, GetFullPath(Path, basePath))
		}
	}
	return out
}
