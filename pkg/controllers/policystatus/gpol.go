package policystatus

import (
	"context"

	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c controller) updateGpolStatus(ctx context.Context, gpol *policiesv1beta1.GeneratingPolicy) error {
	updateFunc := func(gpol *policiesv1beta1.GeneratingPolicy) error {
		p := engineapi.NewGeneratingPolicy(gpol)
		// conditions
		conditionStatus := c.reconcileBeta1Conditions(ctx, p)
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
		// assign
		gpol.Status = policiesv1beta1.GeneratingPolicyStatus{
			ConditionStatus: *conditionStatus,
		}
		return nil
	}
	err := controllerutils.UpdateStatus(
		ctx,
		gpol,
		c.client.PoliciesV1beta1().GeneratingPolicies(),
		updateFunc,
		func(current, expect *policiesv1beta1.GeneratingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	return err
}

func (c controller) updateNGpolStatus(ctx context.Context, ngpol *policiesv1beta1.NamespacedGeneratingPolicy) error {
	updateFunc := func(ngpol *policiesv1beta1.NamespacedGeneratingPolicy) error {
		p := engineapi.NewNamespacedGeneratingPolicy(ngpol)
		// conditions
		conditionStatus := c.reconcileBeta1Conditions(ctx, p)
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
		// assign
		ngpol.Status = policiesv1beta1.GeneratingPolicyStatus{
			ConditionStatus: *conditionStatus,
		}
		return nil
	}
	err := controllerutils.UpdateStatus(ctx,
		ngpol,
		c.client.PoliciesV1beta1().NamespacedGeneratingPolicies(ngpol.GetNamespace()),
		updateFunc,
		func(current, expect *policiesv1beta1.NamespacedGeneratingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	return err
}
