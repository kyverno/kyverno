package policystatus

import (
	"context"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	mpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/mpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c controller) updateMpolStatus(ctx context.Context, mpol *policiesv1beta1.MutatingPolicy) error {
	updateFunc := func(mpol *policiesv1beta1.MutatingPolicy) error {
		p := engineapi.NewMutatingPolicy(mpol)
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
		rules, err := mpolautogen.Autogen(mpol)
		if err != nil {
			return err
		}
		autogenStatus := policiesv1beta1.MutatingPolicyAutogenStatus{
			Configs: rules,
		}
		status := mpol.GetStatus()
		mpol.Status = policiesv1beta1.MutatingPolicyStatus{
			ConditionStatus: *conditionStatus,
			Autogen:         autogenStatus,
			Generated:       status.Generated,
		}
		return nil
	}
	err := controllerutils.UpdateStatus(ctx,
		mpol,
		c.client.PoliciesV1beta1().MutatingPolicies(),
		updateFunc,
		func(current, expect *policiesv1beta1.MutatingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	return err
}

func (c controller) updateNMpolStatus(ctx context.Context, nmpol *policiesv1beta1.NamespacedMutatingPolicy) error {
	updateFunc := func(nmpol *policiesv1beta1.NamespacedMutatingPolicy) error {
		p := engineapi.NewNamespacedMutatingPolicy(nmpol)
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
		rules, err := mpolautogen.Autogen(nmpol)
		if err != nil {
			return err
		}
		autogenStatus := policiesv1beta1.MutatingPolicyAutogenStatus{
			Configs: rules,
		}
		status := nmpol.GetStatus()
		nmpol.Status = policiesv1beta1.MutatingPolicyStatus{
			ConditionStatus: *conditionStatus,
			Autogen:         autogenStatus,
			Generated:       status.Generated,
		}
		return nil
	}
	err := controllerutils.UpdateStatus(ctx,
		nmpol,
		c.client.PoliciesV1beta1().NamespacedMutatingPolicies(nmpol.Namespace),
		updateFunc,
		func(current, expect *policiesv1beta1.NamespacedMutatingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	return err
}
