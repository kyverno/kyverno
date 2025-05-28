package admission

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalPolicy(kind string, raw []byte) (api.GenericPolicy, error) {
	if kind == "ClusterPolicy" {
		var policy *kyvernov1.ClusterPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewKyvernoPolicy(policy), nil
	} else if kind == "Policy" {
		var policy *kyvernov1.Policy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewKyvernoPolicy(policy), nil
	} else if kind == "ValidatingPolicy" {
		var policy *v1alpha1.ValidatingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewValidatingPolicy(policy), nil
	} else if kind == "ImageValidatingPolicy" {
		var policy *v1alpha1.ImageValidatingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewImageValidatingPolicy(policy), nil
	} else if kind == "GeneratingPolicy" {
		var policy *v1alpha1.GeneratingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewGeneratingPolicy(policy), nil
	} else if kind == "DeletingPolicy" {
		var policy *v1alpha1.DeletingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		return api.NewDeletingPolicy(policy), nil
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
