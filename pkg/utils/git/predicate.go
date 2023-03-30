package git

import (
	"io/fs"
	"path/filepath"
)

func IsYaml(file fs.FileInfo) bool {
	if file.IsDir() {
		return false
	}
	ext := filepath.Ext(file.Name())
	return ext == ".yml" || ext == ".yaml"
}
