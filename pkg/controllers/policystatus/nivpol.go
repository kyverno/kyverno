package policystatus

import (
	"context"
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	ivpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (c controller) updateNivpolStatus(ctx context.Context, nivpol *policiesv1beta1.NamespacedImageValidatingPolicy) error {
	updateFunc := func(nivpol *policiesv1beta1.NamespacedImageValidatingPolicy) error {
		p := engineapi.NewNamespacedImageValidatingPolicy(nivpol)
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
		rules, err := ivpolautogen.AutogenNamespaced(nivpol)
		if err != nil {
			return fmt.Errorf("failed to build autogen rules for nivpol %s: %v", nivpol.GetName(), err)
		}
		autogenStatus := policiesv1beta1.ImageValidatingPolicyAutogenStatus{
			Configs: rules,
		}
		// assign
		nivpol.Status = policiesv1beta1.ImageValidatingPolicyStatus{
			ConditionStatus: *conditionStatus,
			Autogen:         autogenStatus,
		}
		return nil
	}
	err := controllerutils.UpdateStatus(ctx,
		nivpol,
		c.client.PoliciesV1beta1().NamespacedImageValidatingPolicies(nivpol.GetNamespace()),
		updateFunc,
		func(current, expect *policiesv1beta1.NamespacedImageValidatingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
	if err != nil && apierrors.IsConflict(err) {
		// Retry on conflict by getting the latest version and trying again
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			objNew, getErr := c.client.PoliciesV1beta1().NamespacedImageValidatingPolicies(nivpol.GetNamespace()).Get(ctx, nivpol.GetName(), metav1.GetOptions{})
			if getErr != nil {
				return getErr
			}
			return controllerutils.UpdateStatus(ctx,
				objNew,
				c.client.PoliciesV1beta1().NamespacedImageValidatingPolicies(nivpol.GetNamespace()),
				updateFunc,
				func(current, expect *policiesv1beta1.NamespacedImageValidatingPolicy) bool {
					return datautils.DeepEqual(current.Status, expect.Status)
				},
			)
		})
		if retryErr != nil {
			return retryErr
		}
		return nil
	}
	return err
}
