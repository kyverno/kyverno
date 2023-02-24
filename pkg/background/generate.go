package background

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) handleGeneratePolicyAbsence(ur *kyvernov1beta1.UpdateRequest) (err error) {
	if !ur.Spec.DeleteDownstream {
		return nil
	}

	logger.V(4).Info("policy no longer exists, deleting the update request and respective resource based on synchronize", "ur", ur.Name, "policy", ur.Spec.Policy)
	var errs []error
	failedDownstreams := []kyvernov1.ResourceSpec{}
	for _, e := range ur.Status.GeneratedResources {
		if err := c.client.DeleteResource(context.TODO(), e.GetAPIVersion(), e.GetKind(), e.GetNamespace(), e.GetName(), false); err != nil && !apierrors.IsNotFound(err) {
			failedDownstreams = append(failedDownstreams, e)
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		logger.Error(multierr.Combine(errs...), "failed to clean up downstream resources on policy deletion, retrying")
		ur.Status.GeneratedResources = failedDownstreams
		ur.Status.State = kyvernov1beta1.Failed
		_, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
	} else {
		ur.Status.State = kyvernov1beta1.Completed
		_, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
	}
	return
}
