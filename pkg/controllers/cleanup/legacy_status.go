package cleanup

import (
	"context"
	"time"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This file holds Kyverno's own legacy CleanupPolicy/ClusterCleanupPolicy status writer, moved
// here verbatim so its allowance to write the /status subresource can be registered, and removed,
// alongside the writer itself: see deprecations.AllowSubresourceForLegacyStatusWriter and
// decision 4 of the #17708 design. Deleting this file when #17710 removes legacy execution
// removes both the writer and its registration in one commit.
//
//nolint:gochecknoinits // registers this binary's legacy status-subresource allowance; see AGENTS.md precedent.
func init() {
	deprecations.AllowSubresourceForLegacyStatusWriter("status")
}

func (c *controller) updateCleanupPolicyStatus(ctx context.Context, policy kyvernov2.CleanupPolicyInterface, namespace string, time time.Time) error {
	switch obj := policy.(type) {
	case *kyvernov2.ClusterCleanupPolicy:
		latest := obj.DeepCopy()
		latest.Status.LastExecutionTime = metav1.NewTime(time)

		new, err := c.kyvernoClient.KyvernoV2().ClusterCleanupPolicies().UpdateStatus(ctx, latest, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		logging.V(3).Info("updated cluster cleanup policy status", "name", policy.GetName(), "status", new.Status)
	case *kyvernov2.CleanupPolicy:
		latest := obj.DeepCopy()
		latest.Status.LastExecutionTime = metav1.NewTime(time)

		new, err := c.kyvernoClient.KyvernoV2().CleanupPolicies(namespace).UpdateStatus(ctx, latest, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		logging.V(3).Info("updated cleanup policy status", "name", policy.GetName(), "namespace", policy.GetNamespace(), "status", new.Status)
	}
	return nil
}
