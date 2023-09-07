package values

import (
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func Load(f billy.Filesystem, filepath string) (*valuesapi.Values, error) {
	yamlBytes, err := readFile(f, filepath)
	if err != nil {
		return nil, err
	}
	vals := &valuesapi.Values{}
	if err := yaml.UnmarshalStrict(yamlBytes, vals); err != nil {
		return nil, err
	}
	return vals, nil
}

func readFile(f billy.Filesystem, filepath string) ([]byte, error) {
	if f != nil {
		file, err := f.Open(filepath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(file)
	}
	return os.ReadFile(filepath)
}
