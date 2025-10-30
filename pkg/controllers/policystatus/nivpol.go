package policystatus

import (
	"context"
	"fmt"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	ivpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c controller) updateNivpolStatus(ctx context.Context, nivpol *policiesv1alpha1.NamespacedImageValidatingPolicy) error {
	updateFunc := func(nivpol *policiesv1alpha1.NamespacedImageValidatingPolicy) error {
		p := engineapi.NewNamespacedImageValidatingPolicy(nivpol)
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
		// autogen
		rules, err := ivpolautogen.AutogenNamespaced(nivpol)
		if err != nil {
			return fmt.Errorf("failed to build autogen rules for nivpol %s: %v", nivpol.GetName(), err)
		}
		autogenStatus := policiesv1alpha1.ImageValidatingPolicyAutogenStatus{
			Configs: rules,
		}
		// assign
		nivpol.Status = policiesv1alpha1.ImageValidatingPolicyStatus{
			ConditionStatus: *conditionStatus,
			Autogen:         autogenStatus,
		}
		return nil
	}
	err := controllerutils.UpdateStatus(ctx,
		nivpol,
		c.client.PoliciesV1alpha1().NamespacedImageValidatingPolicies(nivpol.GetNamespace()),
		updateFunc,
		func(current, expect *policiesv1alpha1.NamespacedImageValidatingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	return err
}
