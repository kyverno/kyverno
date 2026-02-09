package admission

import (
	"fmt"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalPolicy(kind string, raw []byte) (api.GenericPolicy, error) {
	switch kind {
	case "ClusterPolicy":
		var policy *kyvernov1.ClusterPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewKyvernoPolicy(policy), nil
	case "Policy":
		var policy *kyvernov1.Policy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewKyvernoPolicy(policy), nil
	case "ValidatingPolicy":
		var policy *v1beta1.ValidatingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewValidatingPolicy(policy), nil
	case "NamespacedValidatingPolicy":
		var policy *v1beta1.NamespacedValidatingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewNamespacedValidatingPolicy(policy), nil
	case "ImageValidatingPolicy":
		var policy *v1beta1.ImageValidatingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewImageValidatingPolicy(policy), nil
	case "NamespacedImageValidatingPolicy":
		var policy *v1beta1.NamespacedImageValidatingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewNamespacedImageValidatingPolicy(policy), nil
	case "GeneratingPolicy":
		var policy *v1beta1.GeneratingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewGeneratingPolicy(policy), nil
	case "NamespacedGeneratingPolicy":
		var policy *v1beta1.NamespacedGeneratingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewNamespacedGeneratingPolicy(policy), nil
	case "DeletingPolicy":
		var policy *v1beta1.DeletingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewDeletingPolicy(policy), nil
	case "NamespacedDeletingPolicy":
		var policy *v1beta1.NamespacedDeletingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewNamespacedDeletingPolicy(policy), nil
	}
	return nil, fmt.Errorf("admission request does not contain a policy")
}

func GetPolicies(request admissionv1.AdmissionRequest) (api.GenericPolicy, api.GenericPolicy, error) {
	policy, err := UnmarshalPolicy(request.Kind.Kind, request.Object.Raw)
	if err != nil {
		return nil, nil, err
	}
	if request.Operation == admissionv1.Update {
		oldPolicy, err := UnmarshalPolicy(request.Kind.Kind, request.OldObject.Raw)
		return policy, oldPolicy, err
	}
	return policy, nil, nil
}
