package utils

import (
	"encoding/json"
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetPolicy - extracts policies from YAML bytes
func GetPolicy(bytes []byte) (policies []kyverno.PolicyInterface, err error) {
	documents, err := yamlutils.SplitDocuments(bytes)
	if err != nil {
		return nil, err
	}
	for _, thisPolicyBytes := range documents {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to JSON: %v", err)
		}
		policy := &kyverno.ClusterPolicy{}
		if err := json.Unmarshal(policyBytes, policy); err != nil {
			return nil, fmt.Errorf("failed to decode policy: %v", err)
		}
		if policy.TypeMeta.Kind == "" {
			log.Log.V(3).Info("skipping file as policy.TypeMeta.Kind not found")
			continue
		}
		if policy.TypeMeta.Kind != "ClusterPolicy" && policy.TypeMeta.Kind != "Policy" {
			return nil, fmt.Errorf("resource %s/%s is not a Policy or a ClusterPolicy", policy.Kind, policy.Name)
		}
		if policy.Namespace != "" || (policy.Namespace == "" && policy.Kind == "Policy") {
			if policy.Namespace == "" {
				policy.Namespace = "default"
			}
			policy.Kind = "ClusterPolicy"
		}
		policies = append(policies, policy)
	}
	return policies, nil
}
