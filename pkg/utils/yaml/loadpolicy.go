package yaml

import (
	"encoding/json"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	log "github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// GetPolicy extracts policies from YAML bytes
func GetPolicy(bytes []byte) (policies []kyvernov1.PolicyInterface, err error) {
	documents, err := SplitDocuments(bytes)
	if err != nil {
		return nil, err
	}
	for _, thisPolicyBytes := range documents {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to JSON: %v", err)
		}
		policy := &kyvernov1.ClusterPolicy{}
		if err := json.Unmarshal(policyBytes, policy); err != nil {
			return nil, fmt.Errorf("failed to decode policy: %v", err)
		}
		if policy.TypeMeta.Kind == "" {
			log.V(3).Info("skipping file as policy.TypeMeta.Kind not found")
			continue
		}
		if policy.TypeMeta.Kind != "ClusterPolicy" && policy.TypeMeta.Kind != "Policy" {
			return nil, fmt.Errorf("resource %s/%s is not a Policy or a ClusterPolicy", policy.Kind, policy.Name)
		}
		if policy.Kind == "Policy" {
			if policy.Namespace == "" {
				policy.Namespace = "default"
			}
		} else {
			policy.Namespace = ""
		}
		policies = append(policies, policy)
	}
	return policies, nil
}
