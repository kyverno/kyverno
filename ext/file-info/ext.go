package fileinfo

import (
	"io/fs"

	"github.com/kyverno/kyverno/ext/file"
)

func IsYaml(info fs.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	return file.IsYaml(info.Name())
}

func IsJson(info fs.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	return file.IsJson(info.Name())
}

func IsYamlOrJson(info fs.FileInfo) bool {
	if info.IsDir() {
		return false
	}
	return file.IsYamlOrJson(info.Name())
}
