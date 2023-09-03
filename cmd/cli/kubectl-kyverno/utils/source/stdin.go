package source

import (
	"os"
)

var defaultStater = (*os.File).Stat

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

func IsStdin() bool {
	return isStdin(defaultStater)
}
