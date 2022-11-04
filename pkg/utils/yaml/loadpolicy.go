package yaml

import (
	"encoding/json"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	log "github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
		us := &unstructured.Unstructured{}

		if err := json.Unmarshal(policyBytes, us); err != nil {
			return nil, fmt.Errorf("failed to decode policy: %v", err)
		}
		if us.IsList() {
			list, err := us.ToList()
			if err != nil {
				return nil, fmt.Errorf("failed to decode policy list: %v", err)
			}

			for i := range list.Items {
				item := list.Items[i]
				if policies, err = addPolicy(policies, &item); err != nil {
					return nil, err
				}
			}
		} else {
			if policies, err = addPolicy(policies, us); err != nil {
				return nil, err
			}
		}
	}
	return policies, nil
}

func addPolicy(policies []kyvernov1.PolicyInterface, us *unstructured.Unstructured) ([]kyvernov1.PolicyInterface, error) {
	policy := &kyvernov1.ClusterPolicy{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, policy); err != nil {
		return nil, fmt.Errorf("failed to decode policy: %v", err)
	}

	if policy.TypeMeta.Kind == "" {
		log.V(3).Info("skipping file as policy.TypeMeta.Kind not found")
		return policies, nil
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
	return policies, nil
}
