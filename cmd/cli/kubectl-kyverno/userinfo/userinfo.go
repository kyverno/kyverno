package userinfo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
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
			return nil, fmt.Errorf("unable to read yaml (%w)", err)
		}
		return bytes, err
	}
}

func Load(fs billy.Filesystem, path string, resourcePath string) (*v1alpha1.UserInfo, error) {
	bytes, err := load(fs, path, resourcePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read yaml (%w)", err)
	}
	var userInfo v1alpha1.UserInfo
	if err := yaml.UnmarshalStrict(bytes, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode yaml (%w)", err)
	}
	return &userInfo, nil
}
