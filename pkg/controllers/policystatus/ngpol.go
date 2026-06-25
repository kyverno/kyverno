package policystatus

import (
	"context"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c controller) updateNGpolStatus(ctx context.Context, ngpol *v1beta1.NamespacedGeneratingPolicy) error {
	updateFunc := func(ngpol *v1beta1.NamespacedGeneratingPolicy) error {
		p := engineapi.NewNamespacedGeneratingPolicy(ngpol)
		// conditions
		conditionStatus := c.reconcileConditions(ctx, p)
		ready := true
		for _, condition := range conditionStatus.Conditions {
			if condition.Status != metav1.ConditionTrue {
				ready = false
				break
			}
		}
		if conditionStatus.Ready == nil || conditionStatus.IsReady() != ready {
			conditionStatus.Ready = &ready
		}
		// assign - convert v1beta1.ConditionStatus to v1alpha1.ConditionStatus
		ngpol.Status = v1beta1.GeneratingPolicyStatus{
			ConditionStatus: v1beta1.ConditionStatus{
				Conditions: conditionStatus.Conditions,
				Ready:      conditionStatus.Ready,
				Message:    conditionStatus.Message,
			},
		}
		return nil
	}
	err := controllerutils.UpdateStatus(
		ctx,
		ngpol,
		c.client.PoliciesV1beta1().NamespacedGeneratingPolicies(ngpol.GetNamespace()),
		updateFunc,
		func(current, expect *v1beta1.NamespacedGeneratingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	return err
}
