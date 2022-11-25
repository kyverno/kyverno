package policy

import (
	"encoding/json"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"sigs.k8s.io/yaml"
)

func ToJson(policy kyvernov1.PolicyInterface) ([]byte, error) {
	return json.Marshal(policy)
}

func ToYaml(policy kyvernov1.PolicyInterface) ([]byte, error) {
	jsonBytes, err := ToJson(policy)
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(jsonBytes)
}
