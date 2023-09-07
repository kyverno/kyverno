package source

import (
	"os"
)

var defaultStater = (*os.File).Stat

func IsStdin(path string) bool {
	return path == "-" && isStdin(defaultStater)
}

func isStdin(stater func(*os.File) (os.FileInfo, error)) bool {
	if stater == nil {
		stater = defaultStater
	}
	fileInfo, err := stater(os.Stdin)
	if err != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeCharDevice == 0
}
