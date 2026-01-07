package policystatus

import (
	"context"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c controller) updateVpolStatus(ctx context.Context, vpol *policiesv1beta1.ValidatingPolicy) error {
	updateFunc := func(vpol *policiesv1beta1.ValidatingPolicy) error {
		p := engineapi.NewValidatingPolicy(vpol)
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
		// autogen
		rules, err := vpolautogen.Autogen(vpol)
		if err != nil {
			return err
		}
		autogenStatus := policiesv1beta1.ValidatingPolicyAutogenStatus{
			Configs: rules,
		}
		// assign
		status := vpol.GetStatus()
		vpol.Status = policiesv1beta1.ValidatingPolicyStatus{
			ConditionStatus: *conditionStatus,
			Autogen:         autogenStatus,
			Generated:       status.Generated,
		}
		return nil
	}
	err := controllerutils.UpdateStatus(ctx,
		vpol,
		c.client.PoliciesV1beta1().ValidatingPolicies(),
		updateFunc,
		func(current, expect *policiesv1beta1.ValidatingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	return err
}

func (c controller) updateNVpolStatus(ctx context.Context, nvpol *policiesv1beta1.NamespacedValidatingPolicy) error {
	updateFunc := func(vpol *policiesv1beta1.NamespacedValidatingPolicy) error {
		p := engineapi.NewNamespacedValidatingPolicy(vpol)
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
		// autogen
		rules, err := vpolautogen.Autogen(vpol)
		if err != nil {
			return err
		}
		autogenStatus := policiesv1beta1.ValidatingPolicyAutogenStatus{
			Configs: rules,
		}
		// assign
		status := vpol.GetStatus()
		vpol.Status = policiesv1beta1.ValidatingPolicyStatus{
			ConditionStatus: *conditionStatus,
			Autogen:         autogenStatus,
			Generated:       status.Generated,
		}
		return nil
	}
	err := controllerutils.UpdateStatus(ctx,
		nvpol,
		c.client.PoliciesV1beta1().NamespacedValidatingPolicies(nvpol.Namespace),
		updateFunc,
		func(current, expect *policiesv1beta1.NamespacedValidatingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	return err
}
