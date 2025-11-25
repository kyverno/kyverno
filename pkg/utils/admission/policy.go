package admission

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
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
		var policy *v1alpha1.GeneratingPolicy
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, err
		}
		// Convert v1alpha1 to v1beta1 for NewGeneratingPolicy
		var webhookConfig *v1beta1.WebhookConfiguration
		if policy.Spec.WebhookConfiguration != nil {
			webhookConfig = &v1beta1.WebhookConfiguration{
				TimeoutSeconds: policy.Spec.WebhookConfiguration.TimeoutSeconds,
			}
		}
		var evalConfig *v1beta1.GeneratingPolicyEvaluationConfiguration
		if policy.Spec.EvaluationConfiguration != nil {
			evalConfig = &v1beta1.GeneratingPolicyEvaluationConfiguration{
				Admission:                      (*v1beta1.AdmissionConfiguration)(policy.Spec.EvaluationConfiguration.Admission),
				GenerateExistingConfiguration:  (*v1beta1.GenerateExistingConfiguration)(policy.Spec.EvaluationConfiguration.GenerateExistingConfiguration),
				SynchronizationConfiguration:   (*v1beta1.SynchronizationConfiguration)(policy.Spec.EvaluationConfiguration.SynchronizationConfiguration),
				OrphanDownstreamOnPolicyDelete: (*v1beta1.OrphanDownstreamOnPolicyDeleteConfiguration)(policy.Spec.EvaluationConfiguration.OrphanDownstreamOnPolicyDelete),
			}
		}
		generations := make([]v1beta1.Generation, len(policy.Spec.Generation))
		for i, g := range policy.Spec.Generation {
			generations[i] = v1beta1.Generation{Expression: g.Expression}
		}
		v1beta1Pol := &v1beta1.GeneratingPolicy{
			TypeMeta:   policy.TypeMeta,
			ObjectMeta: policy.ObjectMeta,
			Spec: v1beta1.GeneratingPolicySpec{
				MatchConstraints:        policy.Spec.MatchConstraints,
				MatchConditions:         policy.Spec.MatchConditions,
				Variables:               policy.Spec.Variables,
				EvaluationConfiguration: evalConfig,
				WebhookConfiguration:    webhookConfig,
				Generation:              generations,
			},
			Status: v1beta1.GeneratingPolicyStatus{
				ConditionStatus: v1beta1.ConditionStatus{
					Ready:      policy.Status.ConditionStatus.Ready,
					Conditions: policy.Status.ConditionStatus.Conditions,
				},
			},
		}
		v1beta1Pol.TypeMeta.APIVersion = v1beta1.GroupVersion.String()
		v1beta1Pol.TypeMeta.Kind = v1beta1.GeneratingPolicyKind
		return api.NewGeneratingPolicy(v1beta1Pol), nil
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
