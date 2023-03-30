package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func newUR(policy kyvernov1.PolicyInterface, trigger kyvernov1.ResourceSpec, ruleName string, ruleType kyvernov1beta1.RequestType, deleteDownstream bool) *kyvernov1beta1.UpdateRequest {
	var policyNameNamespaceKey string

	if policy.IsNamespaced() {
		policyNameNamespaceKey = policy.GetNamespace() + "/" + policy.GetName()
	} else {
		policyNameNamespaceKey = policy.GetName()
	}

	var label labels.Set
	if ruleType == kyvernov1beta1.Mutate {
		label = common.MutateLabelsSet(policyNameNamespaceKey, trigger)
	} else {
		label = common.GenerateLabelsSet(policyNameNamespaceKey, trigger)
	}

	return &kyvernov1beta1.UpdateRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kyvernov1beta1.SchemeGroupVersion.String(),
			Kind:       "UpdateRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ur-",
			Namespace:    config.KyvernoNamespace(),
			Labels:       label,
		},
		Spec: kyvernov1beta1.UpdateRequestSpec{
			Type:   ruleType,
			Policy: policyNameNamespaceKey,
			Rule:   ruleName,
			Resource: kyvernov1.ResourceSpec{
				Kind:       trigger.GetKind(),
				Namespace:  trigger.GetNamespace(),
				Name:       trigger.GetName(),
				APIVersion: trigger.GetAPIVersion(),
			},
			DeleteDownstream: deleteDownstream,
		},
	}
}

func newURStatus(downstream unstructured.Unstructured) kyvernov1beta1.UpdateRequestStatus {
	return kyvernov1beta1.UpdateRequestStatus{
		State: kyvernov1beta1.Pending,
		GeneratedResources: []kyvernov1.ResourceSpec{
			{
				APIVersion: downstream.GetAPIVersion(),
				Kind:       downstream.GetKind(),
				Namespace:  downstream.GetNamespace(),
				Name:       downstream.GetName(),
			},
		},
	}
}
