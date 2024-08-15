package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newMutateUR(policy kyvernov1.PolicyInterface, trigger kyvernov1.ResourceSpec, ruleName string) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	ur.Labels = common.MutateLabelsSet(policyKey(policy), trigger)
	ur.Spec = kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Mutate,
		Policy: policyKey(policy),
		Rule:   ruleName,
		Resource: kyvernov1.ResourceSpec{
			Kind:       trigger.GetKind(),
			Namespace:  trigger.GetNamespace(),
			Name:       trigger.GetName(),
			APIVersion: trigger.GetAPIVersion(),
			UID:        trigger.GetUID(),
		},
	}
	return ur
}

func newGenerateUR(policy kyvernov1.PolicyInterface) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	ur.Labels = common.GenerateLabelsSet(policyKey(policy))
	ur.Spec = kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Generate,
		Policy: policyKey(policy),
	}
	return ur
}

func newUrMeta() *kyvernov2.UpdateRequest {
	return &kyvernov2.UpdateRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kyvernov2.SchemeGroupVersion.String(),
			Kind:       "UpdateRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ur-",
			Namespace:    config.KyvernoNamespace(),
		},
	}
}

func addGeneratedResources(ur *kyvernov2.UpdateRequest, downstream unstructured.Unstructured) {
	ur.Status.GeneratedResources = append(ur.Status.GeneratedResources,
		kyvernov1.ResourceSpec{
			APIVersion: downstream.GetAPIVersion(),
			Kind:       downstream.GetKind(),
			Namespace:  downstream.GetNamespace(),
			Name:       downstream.GetName(),
			UID:        downstream.GetUID(),
		},
	)
}

func addRuleContext(ur *kyvernov2.UpdateRequest, ruleName string, trigger kyvernov1.ResourceSpec, deleteDownstream bool) {
	ur.Spec.RuleContext = append(ur.Spec.RuleContext, kyvernov2.RuleContext{
		Rule: ruleName,
		Trigger: kyvernov1.ResourceSpec{
			Kind:       trigger.GetKind(),
			Namespace:  trigger.GetNamespace(),
			Name:       trigger.GetName(),
			APIVersion: trigger.GetAPIVersion(),
			UID:        trigger.GetUID(),
		},
		DeleteDownstream: deleteDownstream,
	})
}
