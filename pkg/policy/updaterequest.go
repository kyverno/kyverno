package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func newUR(policy kyvernov1.PolicyInterface, trigger kyvernov1.ResourceSpec, ruleName string, ruleType kyvernov2.RequestType, deleteDownstream bool) *kyvernov2.UpdateRequest {
	var policyNameNamespaceKey string

	if policy.IsNamespaced() {
		policyNameNamespaceKey = policy.GetNamespace() + "/" + policy.GetName()
	} else {
		policyNameNamespaceKey = policy.GetName()
	}

	var label labels.Set
	if ruleType == kyvernov2.Mutate {
		label = common.MutateLabelsSet(policyNameNamespaceKey, trigger)
	} else {
		label = common.GenerateLabelsSet(policyNameNamespaceKey, trigger)
	}

	return &kyvernov2.UpdateRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kyvernov2.SchemeGroupVersion.String(),
			Kind:       "UpdateRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ur-",
			Namespace:    config.KyvernoNamespace(),
			Labels:       label,
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   ruleType,
			Policy: policyNameNamespaceKey,
			Rule:   ruleName,
			Resource: kyvernov1.ResourceSpec{
				Kind:       trigger.GetKind(),
				Namespace:  trigger.GetNamespace(),
				Name:       trigger.GetName(),
				APIVersion: trigger.GetAPIVersion(),
				UID:        trigger.GetUID(),
			},
			DeleteDownstream: deleteDownstream,
		},
	}
}

func newURStatus(downstream unstructured.Unstructured) kyvernov2.UpdateRequestStatus {
	return kyvernov2.UpdateRequestStatus{
		State: kyvernov2.Pending,
		GeneratedResources: []kyvernov1.ResourceSpec{
			{
				APIVersion: downstream.GetAPIVersion(),
				Kind:       downstream.GetKind(),
				Namespace:  downstream.GetNamespace(),
				Name:       downstream.GetName(),
				UID:        downstream.GetUID(),
			},
		},
	}
}
