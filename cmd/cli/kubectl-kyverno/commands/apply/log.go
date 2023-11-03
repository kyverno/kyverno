package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// checkMutateLogPath - checking path for printing mutated resource (-o flag)
func checkMutateLogPath(mutateLogPath string) (mutateLogPathIsDir bool, err error) {
	if mutateLogPath != "" {
		spath := strings.Split(mutateLogPath, "/")
		sfileName := strings.Split(spath[len(spath)-1], ".")
		if sfileName[len(sfileName)-1] == "yml" || sfileName[len(sfileName)-1] == "yaml" {
			mutateLogPathIsDir = false
		} else {
			mutateLogPathIsDir = true
		}

		err := createFileOrFolder(mutateLogPath, mutateLogPathIsDir)
		if err != nil {
			return mutateLogPathIsDir, fmt.Errorf("failed to create file/folder (%w)", err)
		}
	}
	return mutateLogPathIsDir, err
}

// createFileOrFolder - creating file or folder according to path provided
func createFileOrFolder(mutateLogPath string, mutateLogPathIsDir bool) error {
	mutateLogPath = filepath.Clean(mutateLogPath)
	_, err := os.Stat(mutateLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			if !mutateLogPathIsDir {
				// check the folder existence, then create the file
				var folderPath string
				s := strings.Split(mutateLogPath, "/")

				if len(s) > 1 {
					folderPath = mutateLogPath[:len(mutateLogPath)-len(s[len(s)-1])-1]
					_, err := os.Stat(folderPath)
					if os.IsNotExist(err) {
						errDir := os.MkdirAll(folderPath, 0o750)
						if errDir != nil {
							return fmt.Errorf("failed to create directory (%w)", err)
						}
					}
				}

				mutateLogPath = filepath.Clean(mutateLogPath)
				// Necessary for us to create the file via variable as it is part of the CLI.
				file, err := os.OpenFile(mutateLogPath, os.O_RDONLY|os.O_CREATE, 0o600) // #nosec G304
				if err != nil {
					return fmt.Errorf("failed to create file (%w)", err)
				}

				err = file.Close()
				if err != nil {
					return fmt.Errorf("failed to close file (%w)", err)
				}
			} else {
				errDir := os.MkdirAll(mutateLogPath, 0o750)
				if errDir != nil {
					return fmt.Errorf("failed to create directory (%w)", err)
				}
			}
		} else {
			return fmt.Errorf("failed to describe file (%w)", err)
		}
	}

	return nil
}
