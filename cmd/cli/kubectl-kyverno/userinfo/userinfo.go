package userinfo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	sanitizederror "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/sanitizedError"
	"sigs.k8s.io/yaml"
)

func load(fs billy.Filesystem, path string, resourcePath string) ([]byte, error) {
	if fs != nil {
		file, err := fs.Open(filepath.Join(resourcePath, path))
		if err != nil {
			return nil, fmt.Errorf("Unable to open userInfo file: %s. \nerror: %s", path, err)
		}
		bytes, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("Error: failed to read file %s: %w", file.Name(), err)
		}
		return bytes, err
	} else {
		bytes, err := os.ReadFile(filepath.Clean(filepath.Join(resourcePath, path)))
		if err != nil {
			return nil, sanitizederror.NewWithError("unable to read yaml", err)
		}
		return bytes, err
	}
}

func Load(fs billy.Filesystem, path string, resourcePath string) (*kyvernov1beta1.RequestInfo, error) {
	bytes, err := load(fs, path, resourcePath)
	if err != nil {
		return nil, sanitizederror.NewWithError("unable to read yaml", err)
	}
	var userInfo kyvernov1beta1.RequestInfo
	if err := yaml.UnmarshalStrict(bytes, &userInfo); err != nil {
		return nil, sanitizederror.NewWithError("failed to decode yaml", err)
	}
	return &userInfo, nil
}
