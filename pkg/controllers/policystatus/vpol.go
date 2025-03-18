package policystatus

import (
	"context"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c controller) updateVpolStatus(ctx context.Context, vpol *policiesv1alpha1.ValidatingPolicy) error {
	updateFunc := func(vpol *policiesv1alpha1.ValidatingPolicy) error {
		p := engineapi.NewValidatingPolicy(vpol)
		conditionStatus := c.reconcileConditions(ctx, p)

		status := vpol.GetStatus()
		status.Autogen.Rules = nil
		rules := vpolautogen.ComputeRules(vpol)
		status.Autogen.Rules = append(status.Autogen.Rules, rules...)

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
		status.ConditionStatus = *conditionStatus
		return nil
	}

	err := controllerutils.UpdateStatus(ctx,
		vpol,
		c.client.PoliciesV1alpha1().ValidatingPolicies(),
		updateFunc,
		func(current, expect *policiesv1alpha1.ValidatingPolicy) bool {
			if current.GetStatus().GetConditionStatus().Ready == nil || current.GetStatus().GetConditionStatus().IsReady() != expect.GetStatus().GetConditionStatus().IsReady() {
				return false
			}

			if len(current.GetStatus().GetConditionStatus().Conditions) != len(expect.GetStatus().GetConditionStatus().Conditions) {
				return false
			}

			for _, condition := range current.GetStatus().GetConditionStatus().Conditions {
				for _, expectCondition := range expect.GetStatus().GetConditionStatus().Conditions {
					if condition.Type == expectCondition.Type && condition.Status != expectCondition.Status {
						return false
					}
				}
			}
			return datautils.DeepEqual(current.GetStatus().Autogen, expect.GetStatus().Autogen)
		},
	)
	return err
}
