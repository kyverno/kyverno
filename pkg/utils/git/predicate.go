package git

import (
	"io/fs"
	"path/filepath"
	"strings"
)

func IsYaml(file fs.FileInfo) bool {
	if file.IsDir() {
		return false
	}
	ext := strings.ToLower(filepath.Ext(file.Name()))
	return ext == ".yml" || ext == ".yaml"
}
