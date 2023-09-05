// TODO move into the log package
package processor

import (
	"os"
	"path/filepath"

	"github.com/kyverno/kyverno/pkg/logging"
)

var log = logging.WithName("kubectl-kyverno")

func PrintMutatedOutput(mutateLogPath string, mutateLogPathIsDir bool, yaml string, fileName string) error {
	var f *os.File
	var err error
	yaml = yaml + ("\n---\n\n")

	mutateLogPath = filepath.Clean(mutateLogPath)
	if !mutateLogPathIsDir {
		// truncation for the case when mutateLogPath is a file (not a directory) is handled under pkg/kyverno/apply/test_command.go
		f, err = os.OpenFile(mutateLogPath, os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304
	} else {
		f, err = os.OpenFile(mutateLogPath+"/"+fileName+".yaml", os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304
	}

	if err != nil {
		return err
	}
	if _, err := f.Write([]byte(yaml)); err != nil {
		closeErr := f.Close()
		if closeErr != nil {
			log.Error(closeErr, "failed to close file")
		}
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return nil
}
