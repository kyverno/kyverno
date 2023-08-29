package values

import (
	"encoding/json"
	"io"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func readFile(f billy.Filesystem, filepath string) ([]byte, error) {
	if f != nil {
		filep, err := f.Open(filepath)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(filep)
	}
	return os.ReadFile(filepath)
}

func Load(f billy.Filesystem, filepath string) (*api.Values, error) {
	yamlBytes, err := readFile(f, filepath)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := yaml.ToJSON(yamlBytes)
	if err != nil {
		return nil, err
	}
	vals := &api.Values{}
	if err := json.Unmarshal(jsonBytes, vals); err != nil {
		return nil, err
	}
	return vals, nil
}
