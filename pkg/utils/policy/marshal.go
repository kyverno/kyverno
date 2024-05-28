package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"sigs.k8s.io/yaml"
)

// ToJson marshals a policy into corresponding json bytes.
func ToJson(policy kyvernov1.PolicyInterface) ([]byte, error) {
	return jsonutils.Marshal(policy)
}

// ToYaml marshals a policy into corresponding yaml bytes.
// If firsts converts the policy to json because some internal structures have
// custom json marshalling functions, then it converts json to yaml.
func ToYaml(policy kyvernov1.PolicyInterface) ([]byte, error) {
	jsonBytes, err := ToJson(policy)
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(jsonBytes)
}
