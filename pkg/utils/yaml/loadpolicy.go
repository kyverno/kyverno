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
func GetPolicy(bytes []byte) (policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy, validatingAdmissionPolicyBindings []v1alpha1.ValidatingAdmissionPolicyBinding, err error) {
	documents, err := SplitDocuments(bytes)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, thisPolicyBytes := range documents {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to convert to JSON: %v", err)
		}
		us := &unstructured.Unstructured{}

		if err := json.Unmarshal(policyBytes, us); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to decode policy: %v", err)
		}
		if us.IsList() {
			list, err := us.ToList()
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to decode policy list: %v", err)
			}

			for i := range list.Items {
				item := list.Items[i]
				if err = addPolicy(&policies, &validatingAdmissionPolicies, &validatingAdmissionPolicyBindings, &item); err != nil {
					return nil, nil, nil, err
				}
			}
		} else {
			if err = addPolicy(&policies, &validatingAdmissionPolicies, &validatingAdmissionPolicyBindings, us); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	return policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, err
}

func addPolicy(policies *[]kyvernov1.PolicyInterface, validatingAdmissionPolicies *[]v1alpha1.ValidatingAdmissionPolicy, validatingAdmissionPolicyBindings *[]v1alpha1.ValidatingAdmissionPolicyBinding, us *unstructured.Unstructured) error {
	kind := us.GetKind()

	if strings.Compare(kind, "ValidatingAdmissionPolicy") == 0 {
		vap := &v1alpha1.ValidatingAdmissionPolicy{}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, vap); err != nil {
			return fmt.Errorf("failed to decode validating admission policy: %v", err)
		}

		if vap.Kind == "" {
			log.V(3).Info("skipping file as ValidatingAdmissionPolicy.Kind not found")
			return nil
		}

		*validatingAdmissionPolicies = append(*validatingAdmissionPolicies, *vap)
	} else if strings.Compare(kind, "ValidatingAdmissionPolicyBinding") == 0 {
		vapBinding := &v1alpha1.ValidatingAdmissionPolicyBinding{}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, vapBinding); err != nil {
			return fmt.Errorf("failed to decode validating admission policy binding: %v", err)
		}

		if vapBinding.Kind == "" {
			log.V(3).Info("skipping file as ValidatingAdmissionPolicyBinding.Kind not found")
			return nil
		}

		*validatingAdmissionPolicyBindings = append(*validatingAdmissionPolicyBindings, *vapBinding)
	} else {
		var policy kyvernov1.PolicyInterface
		if us.GetKind() == "ClusterPolicy" {
			policy = &kyvernov1.ClusterPolicy{}
		} else {
			policy = &kyvernov1.Policy{}
		}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, policy); err != nil {
			return fmt.Errorf("failed to decode policy: %v", err)
		}

		if policy.GetKind() == "" {
			log.V(3).Info("skipping file as policy.TypeMeta.Kind not found")
			return nil
		}
		if policy.GetKind() != "ClusterPolicy" && policy.GetKind() != "Policy" {
			return fmt.Errorf("resource %s/%s is not a Policy or a ClusterPolicy", policy.GetKind(), policy.GetName())
		}

		if policy.GetKind() == "Policy" {
			if policy.GetNamespace() == "" {
				policy.SetNamespace("default")
			}
		} else {
			policy.SetNamespace("")
		}
		*policies = append(*policies, policy)
	}
	return nil
}
