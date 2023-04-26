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
	var policy kyvernov1.PolicyInterface
	if us.GetKind() == "ClusterPolicy" {
		policy = &kyvernov1.ClusterPolicy{}
	} else {
		policy = &kyvernov1.Policy{}
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, policy); err != nil {
		return nil, fmt.Errorf("failed to decode policy: %v", err)
	}

	if policy.GetKind() == "" {
		log.V(3).Info("skipping file as policy.TypeMeta.Kind not found")
		return policies, nil
	}
	if policy.GetKind() != "ClusterPolicy" && policy.GetKind() != "Policy" {
		return nil, fmt.Errorf("resource %s/%s is not a Policy or a ClusterPolicy", policy.GetKind(), policy.GetName())
	}

	if policy.GetKind() == "Policy" {
		if policy.GetNamespace() == "" {
			policy.SetNamespace("default")
		}
	} else {
		policy.SetNamespace("")
	}
	policies = append(policies, policy)
	return policies, nil
}
