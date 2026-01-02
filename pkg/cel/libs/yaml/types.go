package yaml

import (
	"github.com/google/cel-go/common/types"
	"sigs.k8s.io/yaml"
)

var YamlType = types.NewOpaqueType("yaml.Yaml")

type YamlIface interface {
	Parse([]byte) (any, error)
}

type Yaml struct {
	YamlIface
}

type YamlImpl struct{}

func (y *YamlImpl) Parse(content []byte) (any, error) {
	var v any
	if err := yaml.Unmarshal(content, &v); err != nil {
		return nil, err
	}
	return v, nil
}
