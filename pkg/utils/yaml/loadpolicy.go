package yaml

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	log "github.com/kyverno/kyverno/pkg/logging"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// GetPolicy extracts policies from YAML bytes
func GetPolicy(bytes []byte) (
	policies []kyvernov1.PolicyInterface,
	validatingAdmissionPolicies []admissionregistrationv1beta1.ValidatingAdmissionPolicy,
	validatingAdmissionPolicyBindings []admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding,
	validatingPolicies []kyvernov2alpha1.ValidatingPolicy,
	err error,
) {
	documents, err := extyaml.SplitDocuments(bytes)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, thisPolicyBytes := range documents {
		policyBytes, err := yaml.ToJSON(thisPolicyBytes)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to convert to JSON: %v", err)
		}
		var us unstructured.Unstructured
		if err := us.UnmarshalJSON(policyBytes); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to decode policy: %v", err)
		}
		if us.IsList() {
			list, err := us.ToList()
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to decode policy list: %v", err)
			}
			for i := range list.Items {
				item := list.Items[i]
				vap, vapb, pol, vp, err := parse(item)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				if vap != nil {
					validatingAdmissionPolicies = append(validatingAdmissionPolicies, *vap)
				}
				if vapb != nil {
					validatingAdmissionPolicyBindings = append(validatingAdmissionPolicyBindings, *vapb)
				}
				if pol != nil {
					policies = append(policies, pol)
				}
				if vp != nil {
					validatingPolicies = append(validatingPolicies, *vp)
				}
			}
		} else {
			vap, vapb, pol, vp, err := parse(us)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if vap != nil {
				validatingAdmissionPolicies = append(validatingAdmissionPolicies, *vap)
			}
			if vapb != nil {
				validatingAdmissionPolicyBindings = append(validatingAdmissionPolicyBindings, *vapb)
			}
			if pol != nil {
				policies = append(policies, pol)
			}
			if vp != nil {
				validatingPolicies = append(validatingPolicies, *vp)
			}
		}
	}
	return policies, validatingAdmissionPolicies, validatingAdmissionPolicyBindings, validatingPolicies, err
}

func parse(obj unstructured.Unstructured) (
	*admissionregistrationv1beta1.ValidatingAdmissionPolicy,
	*admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding,
	kyvernov1.PolicyInterface,
	*kyvernov2alpha1.ValidatingPolicy,
	error,
) {
	switch obj.GetKind() {
	case "ValidatingAdmissionPolicy":
		out, err := parseValidatingAdmissionPolicy(obj)
		return out, nil, nil, nil, err
	case "ValidatingAdmissionPolicyBinding":
		out, err := parseValidatingAdmissionPolicyBinding(obj)
		return nil, out, nil, nil, err
	case "Policy":
		out, err := parsePolicy(obj)
		return nil, nil, out, nil, err
	case "ClusterPolicy":
		out, err := parseClusterPolicy(obj)
		return nil, nil, out, nil, err
	case "ValidatingPolicy":
		out, err := parseValidatingPolicy(obj)
		return nil, nil, nil, out, err
	}
	return nil, nil, nil, nil, nil
}

func parseValidatingAdmissionPolicy(obj unstructured.Unstructured) (*admissionregistrationv1beta1.ValidatingAdmissionPolicy, error) {
	var out admissionregistrationv1beta1.ValidatingAdmissionPolicy
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj.Object, &out, true); err != nil {
		return nil, fmt.Errorf("failed to decode policy: %v", err)
	}
	if out.Kind == "" {
		log.V(3).Info("skipping file as ValidatingAdmissionPolicy.Kind not found")
		return nil, nil
	}
	return &out, nil
}

func parseValidatingAdmissionPolicyBinding(obj unstructured.Unstructured) (*admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, error) {
	var out admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj.Object, &out, true); err != nil {
		return nil, fmt.Errorf("failed to decode policy: %v", err)
	}
	if out.Kind == "" {
		log.V(3).Info("skipping file as ValidatingAdmissionPolicyBinding.Kind not found")
		return nil, nil
	}
	return &out, nil
}

func parsePolicy(obj unstructured.Unstructured) (*kyvernov1.Policy, error) {
	var out kyvernov1.Policy
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj.Object, &out, true); err != nil {
		return nil, fmt.Errorf("failed to decode policy: %v", err)
	}
	if out.Kind == "" {
		log.V(3).Info("skipping file as Policy.Kind not found")
		return nil, nil
	}
	if out.GetNamespace() == "" {
		out.SetNamespace("default")
	}
	return &out, nil
}

func parseClusterPolicy(obj unstructured.Unstructured) (*kyvernov1.ClusterPolicy, error) {
	var out kyvernov1.ClusterPolicy
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj.Object, &out, true); err != nil {
		return nil, fmt.Errorf("failed to decode policy: %v", err)
	}
	if out.Kind == "" {
		log.V(3).Info("skipping file as ClusterPolicy.Kind not found")
		return nil, nil
	}
	out.SetNamespace("")
	return &out, nil
}

func parseValidatingPolicy(obj unstructured.Unstructured) (*kyvernov2alpha1.ValidatingPolicy, error) {
	var out kyvernov2alpha1.ValidatingPolicy
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj.Object, &out, true); err != nil {
		return nil, fmt.Errorf("failed to decode policy: %v", err)
	}
	if out.Kind == "" {
		log.V(3).Info("skipping file as ValidatingPolicy.Kind not found")
		return nil, nil
	}
	return &out, nil
}
