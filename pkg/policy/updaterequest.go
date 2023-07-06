package policy

import (
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func newUR(policy kyvernov1.PolicyInterface, trigger kyverno.ResourceSpec, ruleName string, ruleType kyverno.RequestType, deleteDownstream bool) *kyvernov1beta1.UpdateRequest {
	var policyNameNamespaceKey string

	if policy.IsNamespaced() {
		policyNameNamespaceKey = policy.GetNamespace() + "/" + policy.GetName()
	} else {
		policyNameNamespaceKey = policy.GetName()
	}

	var label labels.Set
	if ruleType == kyverno.Mutate {
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
			Resource: kyverno.ResourceSpec{
				Kind:       trigger.GetKind(),
				Namespace:  trigger.GetNamespace(),
				Name:       trigger.GetName(),
				APIVersion: trigger.GetAPIVersion(),
			},
			DeleteDownstream: deleteDownstream,
		},
	}
}

func newURStatus(downstream unstructured.Unstructured) kyverno.UpdateRequestStatus {
	return kyverno.UpdateRequestStatus{
		State: kyverno.Pending,
		GeneratedResources: []kyverno.ResourceSpec{
			{
				APIVersion: downstream.GetAPIVersion(),
				Kind:       downstream.GetKind(),
				Namespace:  downstream.GetNamespace(),
				Name:       downstream.GetName(),
			},
		},
	}
}
