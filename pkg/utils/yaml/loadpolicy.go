package yaml

import (
	"encoding/json"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	log "github.com/kyverno/kyverno/pkg/logging"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// GetPolicy extracts policies from YAML bytes
func GetPolicy(bytes []byte) (policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []admissionregistrationv1beta1.ValidatingAdmissionPolicy, validatingAdmissionPolicyBindings []admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, err error) {
	documents, err := extyaml.SplitDocuments(bytes)
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
				if policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, err = addPolicy(policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, &item); err != nil {
					return nil, nil, nil, err
				}
			}
		} else {
			if policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, err = addPolicy(policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, us); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	return policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, err
}

func addPolicy(policies []kyvernov1.PolicyInterface, validatingAdmissionPolicies []admissionregistrationv1beta1.ValidatingAdmissionPolicy, validatingAdmissionPolicyBindings []admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, us *unstructured.Unstructured) ([]kyvernov1.PolicyInterface, []admissionregistrationv1beta1.ValidatingAdmissionPolicy, []admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, error) {
	kind := us.GetKind()

	if strings.Compare(kind, "ValidatingAdmissionPolicy") == 0 {
		validatingAdmissionPolicy := admissionregistrationv1beta1.ValidatingAdmissionPolicy{}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(us.Object, &validatingAdmissionPolicy, true); err != nil {
			return policies, nil, validatingAdmissionPolicyBindings, fmt.Errorf("failed to decode policy: %v", err)
		}

		if validatingAdmissionPolicy.Kind == "" {
			log.V(3).Info("skipping file as ValidatingAdmissionPolicy.Kind not found")
			return policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, nil
		}

		validatingAdmissionPolicies = append(validatingAdmissionPolicies, validatingAdmissionPolicy)
	} else if strings.Compare(kind, "ValidatingAdmissionPolicyBinding") == 0 {
		validatingAdmissionPolicyBinding := admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding{}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(us.Object, &validatingAdmissionPolicyBinding, true); err != nil {
			return policies, validatingAdmissionPolicies, nil, fmt.Errorf("failed to decode policy: %v", err)
		}

		if validatingAdmissionPolicyBinding.Kind == "" {
			log.V(3).Info("skipping file as ValidatingAdmissionPolicyBinding.Kind not found")
			return policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, nil
		}

		validatingAdmissionPolicyBindings = append(validatingAdmissionPolicyBindings, validatingAdmissionPolicyBinding)
	} else {
		var policy kyvernov1.PolicyInterface
		if us.GetKind() == "ClusterPolicy" {
			policy = &kyvernov1.ClusterPolicy{}
		} else if us.GetKind() == "Policy" {
			policy = &kyvernov1.Policy{}
		} else {
			return policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, nil
		}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(us.Object, policy, true); err != nil {
			return nil, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, fmt.Errorf("failed to decode policy: %v", err)
		}

		if policy.GetKind() == "" {
			log.V(3).Info("skipping file as policy.TypeMeta.Kind not found")
			return policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, nil
		}
		if policy.GetKind() != "ClusterPolicy" && policy.GetKind() != "Policy" {
			return nil, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, fmt.Errorf("resource %s/%s is not a Policy or a ClusterPolicy", policy.GetKind(), policy.GetName())
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

	return policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, nil
}
