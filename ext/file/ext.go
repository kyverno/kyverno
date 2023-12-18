package file

import (
	"path/filepath"
)

func IsYaml(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yml" || ext == ".yaml"
}

func IsJson(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".json"
}

func IsYamlOrJson(path string) bool {
	return IsYaml(path) || IsJson(path)
}
