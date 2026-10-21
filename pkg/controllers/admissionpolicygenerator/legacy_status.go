package admissionpolicygenerator

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This file holds Kyverno's own legacy ClusterPolicy ValidatingAdmissionPolicy-generation status
// writer, extracted out of updatePolicyStatus's mixed legacy/CEL body (controller.go) so its
// allowance to write the /status subresource can be registered, and removed, alongside the
// writer itself: see deprecations.AllowSubresourceForLegacyStatusWriter and decision 4 of the
// #17708 design. The CEL ValidatingPolicy/MutatingPolicy branches of updatePolicyStatus are not
// legacy and stay in controller.go; deleting this file when #17710 removes legacy execution
// removes only the legacy writer and its registration, not the CEL ones.
//
//nolint:gochecknoinits // registers this binary's legacy status-subresource allowance; see AGENTS.md precedent.
func init() {
	deprecations.AllowSubresourceForLegacyStatusWriter("status")
}

func (c *controller) updateLegacyClusterPolicyStatus(ctx context.Context, cpol *kyvernov1.ClusterPolicy, generated bool, msg string) {
	latest := cpol.DeepCopy()
	latest.Status.ValidatingAdmissionPolicy.Generated = generated
	latest.Status.ValidatingAdmissionPolicy.Message = msg

	new, err := c.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(ctx, latest, metav1.UpdateOptions{})
	if err != nil {
		logging.Error(err, "failed to update cluster policy status", "name", cpol.GetName(), "status", latest.Status)
		return
	}
	logging.V(3).Info("updated cluster policy status", "name", cpol.GetName(), "status", new.Status)
}
