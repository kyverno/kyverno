package yaml

import (
	"encoding/json"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	log "github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// GetPolicy extracts policies from YAML bytes
func GetPolicy(bytes []byte) (policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, err error) {
	documents, err := SplitDocuments(bytes)
	if err != nil {
		return nil, nil, err
	}
	for _, thisPolicyBytes := range documents {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert to JSON: %v", err)
		}
		us := &unstructured.Unstructured{}

		if err := json.Unmarshal(policyBytes, us); err != nil {
			return nil, nil, fmt.Errorf("failed to decode policy: %v", err)
		}
		if us.IsList() {
			list, err := us.ToList()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to decode policy list: %v", err)
			}

			for i := range list.Items {
				item := list.Items[i]
				if policies, validatingAdmissionPolicies, err = addPolicy(policies, validatingAdmissionPolicies, &item); err != nil {
					return nil, nil, err
				}
			}
		} else {
			if policies, validatingAdmissionPolicies, err = addPolicy(policies, validatingAdmissionPolicies, us); err != nil {
				return nil, nil, err
			}
		}
	}
	return policies, validatingAdmissionPolicies, nil
}

func addPolicy(policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, us *unstructured.Unstructured) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	kind := us.GetKind()

	if strings.Compare(kind, "ValidatingAdmissionPolicy") == 0 {
		validatingAdmissionPolicy := v1alpha1.ValidatingAdmissionPolicy{}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(us.Object, &validatingAdmissionPolicy, true); err != nil {
			return policies, nil, fmt.Errorf("failed to decode policy: %v", err)
		}

		if validatingAdmissionPolicy.Kind == "" {
			log.V(3).Info("skipping file as ValidatingAdmissionPolicy.Kind not found")
			return policies, validatingAdmissionPolicies, nil
		}

		validatingAdmissionPolicies = append(validatingAdmissionPolicies, validatingAdmissionPolicy)
	} else {
		var policy kyvernov1.PolicyInterface
		if us.GetKind() == "ClusterPolicy" {
			policy = &kyvernov1.ClusterPolicy{}
		} else if us.GetKind() == "Policy" {
			policy = &kyvernov1.Policy{}
		} else {
			return policies, validatingAdmissionPolicies, nil
		}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(us.Object, policy, true); err != nil {
			return nil, validatingAdmissionPolicies, fmt.Errorf("failed to decode policy: %v", err)
		}

		if policy.GetKind() == "" {
			log.V(3).Info("skipping file as policy.TypeMeta.Kind not found")
			return policies, validatingAdmissionPolicies, nil
		}
		if policy.GetKind() != "ClusterPolicy" && policy.GetKind() != "Policy" {
			return nil, validatingAdmissionPolicies, fmt.Errorf("resource %s/%s is not a Policy or a ClusterPolicy", policy.GetKind(), policy.GetName())
		}

		if policy.GetKind() == "Policy" {
			if policy.GetNamespace() == "" {
				policy.SetNamespace("default")
			}
		} else {
			policy.SetNamespace("")
		}
		policies = append(policies, policy)
	}

	return policies, validatingAdmissionPolicies, nil
}
