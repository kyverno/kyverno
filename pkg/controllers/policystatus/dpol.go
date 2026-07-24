package policystatus

import (
	"context"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reconcileDeletingConditions builds the ConditionStatus for a deleting policy.
//
// Unlike admission-based policies, DeletingPolicy/NamespacedDeletingPolicy are
// driven by a schedule rather than a webhook, so there is no WebhookConfigured
// condition to report. The relevant signal is whether Kyverno can observe the
// resources targeted by the policy's match constraints. The existing status is
// passed in so conditions are merged (and their transition times preserved)
// instead of being reset on every reconcile.
//
// Limitation: permissionsCheck evaluates get/list/watch as the reports service
// account (the subject this controller's authChecker is bound to), which is the
// same signal used for the other CEL policy types. It does NOT verify that the
// cleanup controller's service account holds the "delete" verb on the target
// resources — deletion runs in a separate binary under a different SA. So this
// condition reflects resource observability, not deletion capability; the
// messages below are intentionally worded to that effect. Reporting true
// deletion-readiness would require checking the "delete" verb as the cleanup
// controller SA and is tracked as a follow-up.
func (c controller) reconcileDeletingConditions(ctx context.Context, matchConstraints *admissionregistrationv1.MatchResources, current v1beta1.ConditionStatus) v1beta1.ConditionStatus {
	status := *current.DeepCopy()

	var rules []admissionregistrationv1.NamedRuleWithOperations
	if matchConstraints != nil {
		rules = matchConstraints.ResourceRules
	}
	gvrs := c.resolveGVRs(rules)
	if errs := c.permissionsCheck(ctx, gvrs); len(errs) != 0 {
		status.SetReadyByCondition(v1beta1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionFalse, "Kyverno cannot access the resources targeted by this policy, missing permissions.")
	} else {
		status.SetReadyByCondition(v1beta1.PolicyConditionTypeRBACPermissionsGranted, metav1.ConditionTrue, "Kyverno can access the resources targeted by this policy.")
	}

	ready := true
	for _, condition := range status.Conditions {
		if condition.Status != metav1.ConditionTrue {
			ready = false
			break
		}
	}
	if status.Ready == nil || status.IsReady() != ready {
		status.Ready = &ready
	}
	return status
}

func (c controller) updateDpolStatus(ctx context.Context, dpol *v1beta1.DeletingPolicy) error {
	updateFunc := func(dpol *v1beta1.DeletingPolicy) error {
		// Only touch ConditionStatus; LastExecutionTime is owned by the deleting
		// controller and must be preserved to avoid the two writers clobbering
		// each other's field.
		dpol.Status.ConditionStatus = c.reconcileDeletingConditions(ctx, dpol.Spec.MatchConstraints, dpol.Status.ConditionStatus)
		return nil
	}
	return controllerutils.UpdateStatus(
		ctx,
		dpol,
		c.client.PoliciesV1beta1().DeletingPolicies(),
		updateFunc,
		func(current, expect *v1beta1.DeletingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
}

func (c controller) updateNDpolStatus(ctx context.Context, ndpol *v1beta1.NamespacedDeletingPolicy) error {
	updateFunc := func(ndpol *v1beta1.NamespacedDeletingPolicy) error {
		ndpol.Status.ConditionStatus = c.reconcileDeletingConditions(ctx, ndpol.Spec.MatchConstraints, ndpol.Status.ConditionStatus)
		return nil
	}
	return controllerutils.UpdateStatus(
		ctx,
		ndpol,
		c.client.PoliciesV1beta1().NamespacedDeletingPolicies(ndpol.GetNamespace()),
		updateFunc,
		func(current, expect *v1beta1.NamespacedDeletingPolicy) bool {
			return datautils.DeepEqual(current.Status, expect.Status)
		},
	)
}
