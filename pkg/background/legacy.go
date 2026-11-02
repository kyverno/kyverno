package background

import (
	"context"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// dropLegacyUpdateRequest deletes a mutate/generate UpdateRequest left behind by a
// release that still executed kyverno.io/v1 policies; 1.20 never processes them.
func (c *controller) dropLegacyUpdateRequest(ur *kyvernov2.UpdateRequest) error {
	deleteDownstream := false
	for _, rc := range ur.Spec.RuleContext {
		deleteDownstream = deleteDownstream || rc.DeleteDownstream
	}
	logger.V(2).Info("deleting legacy update request; kyverno.io/v1 policies are not executed in 1.20",
		"name", ur.GetName(), "type", ur.Spec.GetRequestType(), "policy", ur.Spec.GetPolicyKey(),
		"rule", ur.Spec.GetRuleName(), "resource", ur.Spec.GetResource().String(),
		"ruleContexts", len(ur.Spec.RuleContext), "state", ur.Status.State,
		"retryCount", ur.Status.RetryCount, "deleteDownstream", deleteDownstream,
		"generatedResources", len(ur.Status.GeneratedResources))
	err := c.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Delete(
		context.TODO(), ur.GetName(), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
